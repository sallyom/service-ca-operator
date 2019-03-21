package operator

import (
	"fmt"

	"github.com/golang/glog"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
)

func (c *serviceCAOperator) setFailingTrue(operatorConfig *operatorv1.ServiceCA, reason, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})
}

func (c *serviceCAOperator) setFailingFalse(operatorConfig *operatorv1.ServiceCA, reason string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:   operatorv1.OperatorStatusTypeFailing,
			Status: operatorv1.ConditionFalse,
			Reason: reason,
		})
}

func (c *serviceCAOperator) setProgressingTrue(operatorConfig *operatorv1.ServiceCA, reason, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:    operatorv1.OperatorStatusTypeProgressing,
		Status:  operatorv1.ConditionTrue,
		Reason:  reason,
		Message: message,
	})
}

func (c *serviceCAOperator) setAvailableTrue(operatorConfig *operatorv1.ServiceCA, reason string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeAvailable,
		Status: operatorv1.ConditionTrue,
		Reason: reason,
	})
}

func (c *serviceCAOperator) setProgressingFalse(operatorConfig *operatorv1.ServiceCA, reason, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:    operatorv1.OperatorStatusTypeProgressing,
		Status:  operatorv1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

func (c *serviceCAOperator) setAvailableFalse(operatorConfig *operatorv1.ServiceCA, reason, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:    operatorv1.OperatorStatusTypeAvailable,
		Status:  operatorv1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

func isDeploymentStatusAvailable(deploy *appsv1.Deployment) bool {
	return deploy.Status.AvailableReplicas > 0
}

// isDeploymentStatusAvailableAndUpdated returns true when at least one
// replica instance exists and all replica instances are current,
// there are no replica instances remaining from the previous deployment.
// There may still be additional replica instances being created.
func isDeploymentStatusAvailableAndUpdated(deploy *appsv1.Deployment) bool {
	return deploy.Status.AvailableReplicas > 0 &&
		deploy.Status.ObservedGeneration >= deploy.Generation &&
		deploy.Status.UpdatedReplicas == deploy.Status.Replicas
}

func isDeploymentStatusComplete(deploy *appsv1.Deployment) bool {
	replicas := int32(1)
	if deploy.Spec.Replicas != nil {
		replicas = *(deploy.Spec.Replicas)
	}
	return deploy.Status.UpdatedReplicas == replicas &&
		deploy.Status.Replicas == replicas &&
		deploy.Status.AvailableReplicas == replicas &&
		deploy.Status.ObservedGeneration >= deploy.Generation
}

func (c *serviceCAOperator) syncStatus(operatorConfigCopy *operatorv1.ServiceCA, deployments []string) (bool, error) {
	version_ready := 0
	existingDeploymentsAndReplicas := 0
	deployment_complete := 0
	statusMsg := ""
	for _, dep := range deployments {
		reason := "ManagedDeploymentsNotReady"
		existing, err := c.appsv1Client.Deployments(operatorclient.TargetNamespace).Get(dep, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				statusMsg = fmt.Sprintf("Deployment %s does not exist", dep)
				c.setProgressingTrue(operatorConfigCopy, reason, statusMsg)
				// If there isn't at least one replica from each deployment, Available=False
				c.setAvailableFalse(operatorConfigCopy, reason, statusMsg)
				return false, nil
			}
			statusMsg = fmt.Sprintf("Error getting deployment %s", dep)
			c.setFailingTrue(operatorConfigCopy, statusMsg, err.Error())
			// If there isn't at least one replica from each deployment, Available=False
			c.setAvailableFalse(operatorConfigCopy, reason, statusMsg)
			return false, err
		}
		if existing.DeletionTimestamp != nil {
			statusMsg = fmt.Sprintf("Deployment %s is being deleted", dep)
			c.setProgressingTrue(operatorConfigCopy, reason, statusMsg)
			// If there isn't at least one replica from each deployment, Available=False
			c.setAvailableFalse(operatorConfigCopy, reason, statusMsg)
			return false, nil
		}
		if !isDeploymentStatusAvailable(existing) {
			statusMsg = fmt.Sprintf("Deployment %s does not have available replicas", dep)
			c.setProgressingTrue(operatorConfigCopy, reason, statusMsg)
			// If there isn't at least one replica from each deployment, Available=False
			c.setAvailableFalse(operatorConfigCopy, reason, statusMsg)
			return false, nil
		}
		existingDeploymentsAndReplicas++

		if isDeploymentStatusComplete(existing) {
			glog.Infof("Deployment %s has desired replicas.", dep)
			deployment_complete++
		} else {
			statusMsg = fmt.Sprintf("Deployment %s is creating replicas.", dep)
		}
		if isDeploymentStatusAvailableAndUpdated(existing) {
			glog.Infof("Deployment %s is available and updated", dep)
			version_ready++
		} else {
			statusMsg = fmt.Sprintf("Deployment %s is updating", dep)
		}
	}
	// Available, Updated, and Ready to report version:
	// Here, ready to report version and set Available=True and
	// set Progressing=False because all deployments, replicas exist
	// and all instances are updated, no previous deployment instances exist
	if deployment_complete == len(deployments) {
		reason := "ManagedDeploymentsCompleteAndUpdated"
		c.setAvailableTrue(operatorConfigCopy, reason)
		c.setProgressingFalse(operatorConfigCopy, reason, "All service-ca-operator deployments updated")
		return true, nil
	}
	// Step down in readiness:
	// Here, ready to report a version and set Available=True
	// because all deployments exist and at least 1 replica exists per deployment,
	// and there are no instances from previous deployments.
	// Progressing will be true because deployments aren't complete.
	if version_ready == len(deployments) {
		reason := "ManagedDeploymentsAvailableAndUpdated"
		c.setAvailableTrue(operatorConfigCopy, reason)
		c.setProgressingTrue(operatorConfigCopy, reason, statusMsg)
		return true, nil
	}
	// Further step down in readiness:
	// Here, ready to report Available=True
	// because all deployments exist and at least 1 replicas exists per deployment.
	// Don't report version here because
	// there are replica instances remaining from previous deployment
	// Progressing will be true here
	if existingDeploymentsAndReplicas == len(deployments) {
		reason := "ManagedDeploymentsAvailable"
		c.setAvailableTrue(operatorConfigCopy, reason)
		c.setProgressingTrue(operatorConfigCopy, reason, statusMsg)
		return false, nil
	}
	return false, nil
}

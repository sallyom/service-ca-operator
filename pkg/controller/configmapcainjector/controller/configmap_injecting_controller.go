package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/openshift/service-ca-operator/pkg/boilerplate/controller"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
)

// ConfigMapCABundleInjectionController is responsible for injecting a CA bundle into configMaps annotated with
// "service.alpha.openshift.io/inject-cabundle"
type configMapCABundleInjectionController struct {
	configMapClient kcoreclient.ConfigMapsGetter
	configMapLister listers.ConfigMapLister

	ca string
}

func NewConfigMapCABundleInjectionController(configMaps informers.ConfigMapInformer, configMapsClient kcoreclient.ConfigMapsGetter, ca string) controller.Runner {
	ic := &configMapCABundleInjectionController{
		configMapClient: configMapsClient,
		configMapLister: configMaps.Lister(),
		ca:              ca,
	}

	return controller.New("ConfigMapCABundleInjectionController", ic,
		controller.WithInformer(configMaps, controller.FilterFuncs{
			AddFunc:    api.HasInjectCABundleAnnotation,
			UpdateFunc: api.HasInjectCABundleAnnotationUpdate,
		}),
	)
}

func (ic *configMapCABundleInjectionController) Key(namespace, name string) (v1.Object, error) {
	return ic.configMapLister.ConfigMaps(namespace).Get(name)
}

func (ic *configMapCABundleInjectionController) Sync(obj v1.Object) error {
	sharedConfigMap := obj.(*corev1.ConfigMap)

	// check if we need to do anything
	if !api.HasInjectCABundleAnnotation(sharedConfigMap) {
		return nil
	}
	// ensure data of configmap
	return ic.ensureConfigMapCABundleInjection(sharedConfigMap)
}

// ensureConfigMapCABundleInjection will create or update configmap for the
// CA bundle injection as appropriate.
func (ic *configMapCABundleInjectionController) ensureConfigMapCABundleInjection(current *corev1.ConfigMap) error {
	// make a copy to avoid mutating cache state
	configMapCopy := current.DeepCopy()
	// skip updating when the CA bundle is already there
	if data, ok := configMapCopy.Data[api.InjectionDataKey]; ok &&
		data == ic.ca && len(configMapCopy.Data) == 1 {
		return nil
	}
	configMapCopy.Data = map[string]string{api.InjectionDataKey: ic.ca}
	_, err := ic.configMapClient.ConfigMaps(current.Namespace).Update(configMapCopy)
	return err
}

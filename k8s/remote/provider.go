package remote_provider

import (
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/seibert-media/k8s-deploy/k8s"
	k8s_meta "k8s.io/apimachinery/pkg/api/meta"
	k8s_metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8s_runtime "k8s.io/apimachinery/pkg/runtime"
	k8s_schema "k8s.io/apimachinery/pkg/runtime/schema"
	k8s_discovery "k8s.io/client-go/discovery"
	k8s_dynamic "k8s.io/client-go/dynamic"
	k8s_restclient "k8s.io/client-go/rest"
)

type provider struct {
	config *k8s_restclient.Config
}

func New(config *k8s_restclient.Config) k8s.Provider {
	return &provider{
		config: config,
	}
}

func (p *provider) GetObjects(namespace k8s.Namespace) ([]k8s_runtime.Object, error) {
	var result []k8s_runtime.Object
	glog.V(4).Infof("get objects from k8s for namespace %s", namespace)
	discoveryClient, err := k8s_discovery.NewDiscoveryClientForConfig(p.config)
	if err != nil {
		return nil, errors.Wrap(err, "creating k8s_discovery client failed")
	}
	dynamicClientPool := k8s_dynamic.NewDynamicClientPool(p.config)
	resources, err := discoveryClient.ServerResources()
	if err != nil {
		return nil, errors.Wrap(err, "get server resources failed")
	}

	resources = k8s_discovery.FilteredBy(
		k8s_discovery.ResourcePredicateFunc(func(groupVersion string, r *k8s_metav1.APIResource) bool {
			return k8s_discovery.SupportsAllVerbs{Verbs: []string{"list", "create"}}.Match(groupVersion, r)
		}),
		resources,
	)

	handeled := make(map[string]struct{})

	for _, list := range resources {
		glog.V(6).Infof("list group version %v", list.GroupVersion)
		for _, resource := range list.APIResources {

			if _, ok := handeled[resource.Kind]; ok {
				continue
			}
			handeled[resource.Kind] = struct{}{}

			//if resource.Kind != "Deployment" {
			//	continue
			//}

			groupVersion, err := k8s_schema.ParseGroupVersion(list.GroupVersion)
			if err != nil {
				return nil, errors.Wrapf(err, "parse group version %s failed", list.GroupVersion)
			}
			groupVersionKind := groupVersion.WithKind(resource.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "get group version for kind %s failed", resource.Name)
			}

			client, err := dynamicClientPool.ClientForGroupVersionKind(groupVersionKind)
			if err != nil {
				return nil, errors.Wrap(err, "get client for group")
			}

			ri := client.Resource(&resource, namespace.String())

			unstructuredList, err := ri.List(k8s_metav1.ListOptions{})
			if err != nil {
				glog.V(4).Infof("list failed: %v", err)
				continue
			}

			items, err := k8s_meta.ExtractList(unstructuredList)
			if err != nil {
				glog.V(4).Infof("extract items failed: %v", err)
				continue
			}

			for _, item := range items {
				glog.V(6).Infof("found api object %s", k8s.ObjectToString(item))
				is, err := IsManaged(namespace, item)
				if err != nil {
					return nil, errors.Wrap(err, "failed to determine managed state")
				}
				if is {
					result = append(result, item)
				}
			}
		}
	}
	glog.V(1).Infof("read api completed. found %d objects in namespace %s", len(result), namespace)
	return result, nil
}

// IsManaged by k8s-deploy
func IsManaged(namespace k8s.Namespace, object k8s_runtime.Object) (bool, error) {
	u, err := k8s_runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return false, errors.New("unable to convert object to k8s_unstructured")
	}
	obj := &k8s_unstructured.Unstructured{
		Object: u,
	}

	if obj.GetKind() == "Namespace" {
		if obj.GetName() != namespace.String() {
			return false, nil
		}
	}

	if strings.HasPrefix(obj.GetKind(), "ClusterRole") {
		if obj.GetName() == "system:kube-dns-autoscaler" {
			return false, nil
		}
	}

	for _, kind := range []string{"Node", "CertificateSigningRequest", "Event", "ServiceAccount"} {
		if obj.GetKind() == kind {
			return false, nil
		}
	}

	if len(obj.GetOwnerReferences()) > 0 {
		return false, nil
	}

	// Labels
	if _, ok := obj.GetLabels()["kubernetes.io/bootstrapping"]; ok {
		return false, nil
	}
	if _, ok := obj.GetLabels()["kubernetes.io/cluster-service"]; ok {
		return false, nil
	}
	if _, ok := obj.GetLabels()["kube-aggregator.kubernetes.io/automanaged"]; ok {
		return false, nil
	}

	// Annotations
	if _, ok := obj.GetAnnotations()["pv.kubernetes.io/provisioned-by"]; ok {
		return false, nil
	}
	if v, ok := obj.GetAnnotations()["kubernetes.io/service-account.name"]; ok && v == "default" {
		return false, nil
	}

	return true, nil
}

package remote_provider

import (
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/seibert-media/k8s-deploy/k8s"
	k8s_metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	/*clientSet, err := k8s_kubernetes.NewForConfig(p.config)
	if err != nil {
		return nil, errors.Wrap(err, "create clientSet failed: %v")
	}

	var result []k8s_runtime.Object

	ns, err := clientSet.CoreV1().Namespaces().Get(namespace.String(), k8s_metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get namespace failed: %v")
	}

	result = append(result, ns)

	deploymentList, err := clientSet.AppsV1().Deployments(namespace.String()).List(k8s_metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list deployments failed: %v")
	}
	for _, d := range deploymentList.Items {
		var obj = &d
		glog.V(2).Infof("found remote object %s", k8s.ObjectToString(obj))
		result = append(result, obj)
	}
	glog.V(1).Infof("read remote completed. found %d objects", len(result))
	return result, nil*/
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

	for _, list := range resources {
		for _, resource := range list.APIResources {

			gv, err := schema.ParseGroupVersion(list.GroupVersion)
			if err != nil {
				return nil, errors.Wrap(err, "parse group version")
			}
			gvr := gv.WithResource(resource.Name)
			if err != nil {
				return nil, errors.Wrap(err, "parse group version resource")
			}

			client, err := dynamicClientPool.ClientForGroupVersionResource(gvr)
			if err != nil {
				return nil, errors.Wrap(err, "get client for group")
			}

			ri := client.Resource(&resource, namespace.String())

			object, err := ri.List(k8s_metav1.ListOptions{})
			if err != nil {
				glog.V(4).Infof("list failed: %v", err)
				continue
			}

			result = append(result, object)
		}
	}
	glog.V(1).Infof("read api completed. found %d objects in namespace %s", len(result), namespace)
	return result, nil
}

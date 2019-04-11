package uclient

import (
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type Client struct {
	cfg *rest.Config
}

func NewForConfig(cfg *rest.Config) *Client {
	c := &Client{
		cfg: cfg,
	}

	return c
}

func (c *Client) ForUnstructured(u *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	return c.ClientFor(u.GetAPIVersion(), u.GetKind(), u.GetNamespace())
}

func (c *Client) ClientFor(apiVersion, kind, namespace string) (dynamic.ResourceInterface, error) {
	apiResourceList, apiResource, err := c.getAPIResource(apiVersion, kind)
	if err != nil {
		return nil, errors.Wrapf(err, "discovering resource information failed for %s in %s", kind, apiVersion)
	}

	dc, err := newForConfig(apiResourceList.GroupVersion, c.cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "creating dynamic client failed for %s", apiResourceList.GroupVersion)
	}

	gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing GroupVersion failed %s", apiResourceList.GroupVersion)
	}

	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: apiResource.Name,
	}

	return dc.Resource(gvr).Namespace(namespace), nil
}

func (c *Client) getAPIResource(apiVersion, kind string) (*metav1.APIResourceList, *metav1.APIResource, error) {
	kclient, err := kubernetes.NewForConfig(c.cfg)
	if err != nil {
		return nil, nil, err
	}

	apiResourceLists, err := kclient.Discovery().ServerResources()
	if err != nil {
		return nil, nil, err
	}

	for _, apiResourceList := range apiResourceLists {
		if apiResourceList.GroupVersion == apiVersion {
			for _, r := range apiResourceList.APIResources {
				if r.Kind == kind {
					return apiResourceList, &r, nil
				}
			}
		}
	}

	return nil, nil, fmt.Errorf("apiVersion %s and kind %s not found available in Kubernetes cluster", apiVersion, kind)
}

func newForConfig(groupVersion string, c *rest.Config) (dynamic.Interface, error) {
	config := rest.CopyConfig(c)
	err := setConfigDefaults(groupVersion, config)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func setConfigDefaults(groupVersion string, config *rest.Config) error {
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return err
	}
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	if config.GroupVersion.Group == "" && config.GroupVersion.Version == "v1" {
		config.APIPath = "/api"
	}
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	return nil
}

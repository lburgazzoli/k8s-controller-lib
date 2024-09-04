package client

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DiscoveryLimiterBurst = 30
)

var scaleConverter = scale.NewScaleConverter()
var codecs = serializer.NewCodecFactory(scaleConverter.Scheme())

func New(cfg *rest.Config, cc ctrl.Client) (*Client, error) {
	discoveryCl, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Discovery client: %w", err)
	}

	kubeCl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Kubernetes client: %w", err)
	}

	restCl, err := newRESTClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a REST client: %w", err)
	}

	dynCl, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Dynamic client: %w", err)
	}

	apiextCl, err := apiextv1.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct an API Extension client: %w", err)
	}

	c := Client{
		Client:                   cc,
		Interface:                kubeCl,
		ApiextensionsV1Interface: apiextCl,
		Discovery:                discoveryCl,
		dynamic:                  dynCl,
		config:                   cfg,
		rest:                     restCl,
	}

	c.discoveryLimiter = rate.NewLimiter(rate.Every(time.Second), DiscoveryLimiterBurst)
	c.discoveryCache = memory.NewMemCacheClient(discoveryCl)

	return &c, nil
}

type Client struct {
	ctrl.Client
	kubernetes.Interface
	apiextv1.ApiextensionsV1Interface

	Discovery discovery.DiscoveryInterface

	dynamic          *dynamic.DynamicClient
	config           *rest.Config
	rest             rest.Interface
	discoveryCache   discovery.CachedDiscoveryInterface
	discoveryLimiter *rate.Limiter
}

func (c *Client) Dynamic(obj *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	if c.discoveryLimiter.Allow() {
		c.discoveryCache.Invalidate()
	}

	c.discoveryCache.Fresh()

	mapping, err := c.RESTMapper().RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to identify preferred resource mapping for %s/%s: %w",
			obj.GroupVersionKind().GroupKind(),
			obj.GroupVersionKind().Version,
			err)
	}

	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = &NamespacedResource{
			ResourceInterface: c.dynamic.Resource(mapping.Resource),
		}
	} else {
		dr = &ClusteredResource{
			ResourceInterface: c.dynamic.Resource(mapping.Resource),
		}
	}

	return dr, nil
}

func (c *Client) Invalidate() {
	if c.discoveryCache != nil {
		c.discoveryCache.Invalidate()
	}
}

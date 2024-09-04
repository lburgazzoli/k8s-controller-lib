package client

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func newRESTClientForConfig(config *rest.Config) (*rest.RESTClient, error) {
	cfg := rest.CopyConfig(config)
	// so that the RESTClientFor doesn't complain
	cfg.GroupVersion = &schema.GroupVersion{}
	cfg.NegotiatedSerializer = codecs.WithoutConversion()

	if len(cfg.UserAgent) == 0 {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	rc, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a REST client: %w", err)
	}

	return rc, nil
}

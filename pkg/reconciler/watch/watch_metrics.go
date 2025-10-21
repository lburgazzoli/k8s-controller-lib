//nolint:gochecknoglobals,gochecknoinits
package watch

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	WatchedResourcesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "watched_resources",
			Help: "Number of watched resources per controller and GVK",
		},
		[]string{
			"controller",
			"api_version",
			"kind",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(WatchedResourcesTotal)
}

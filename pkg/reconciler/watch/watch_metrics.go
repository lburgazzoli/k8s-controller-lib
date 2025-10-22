//nolint:gochecknoglobals,gochecknoinits
package watch

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricNameWatched         = "dynamic_watched_resources"
	MetricLabelNameController = "controller"
	MetricLabelNameAPIVersion = "api_version"
	MetricLabelNameKind       = "kind"
)

var (
	DynamicWatchedResourcesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNameWatched,
			Help: "Number of watched resources per controller and GVK",
		},
		[]string{
			MetricLabelNameController,
			MetricLabelNameAPIVersion,
			MetricLabelNameKind,
		},
	)
)

func init() {
	metrics.Registry.MustRegister(DynamicWatchedResourcesTotal)
}

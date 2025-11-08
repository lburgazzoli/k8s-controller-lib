//nolint:gochecknoglobals,gochecknoinits
package watch

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// MetricNameWatched is the Prometheus metric name for tracking dynamically watched resources.
	MetricNameWatched = "dynamic_watched_resources"
	// MetricLabelNameController is the label name representing the controller name in Prometheus metrics.
	MetricLabelNameController = "controller"
	// MetricLabelNameAPIVersion is the Prometheus label for watched resource API version.
	MetricLabelNameAPIVersion = "api_version"
	// MetricLabelNameKind is the Prometheus label for watched resource kind.
	MetricLabelNameKind = "kind"
)

var (
	// DynamicWatchedResourcesTotal is a Prometheus gauge tracking the number of watched resources per controller and GVK.
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

//nolint:gochecknoglobals,gochecknoinits
package watch

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// MetricNameDynamicWatched is the Prometheus metric name for tracking dynamically watched resources.
	MetricNameDynamicWatched = "dynamic_watched_resources"
	// MetricNameStaticWatched is the Prometheus metric name for tracking statically watched resources.
	MetricNameStaticWatched = "static_watched_resources"
	// MetricLabelNameController is the label name representing the controller name in Prometheus metrics.
	MetricLabelNameController = "controller"
	// MetricLabelNameAPIVersion is the Prometheus label for watched resource API version.
	MetricLabelNameAPIVersion = "api_version"
	// MetricLabelNameKind is the Prometheus label for watched resource kind.
	MetricLabelNameKind = "kind"
)

var (
	// DynamicWatchedResourcesTotal is a Prometheus gauge tracking the number of dynamically watched resources
	// per controller and GVK. These are watches registered by the auto-watch system during reconciliation.
	DynamicWatchedResourcesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNameDynamicWatched,
			Help: "Number of dynamically watched resources per controller and GVK",
		},
		[]string{
			MetricLabelNameController,
			MetricLabelNameAPIVersion,
			MetricLabelNameKind,
		},
	)

	// StaticWatchedResourcesTotal is a Prometheus gauge tracking the number of statically watched resources
	// per controller and GVK. These are watches registered by Builder via For(), Owns(), or Watches().
	StaticWatchedResourcesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricNameStaticWatched,
			Help: "Number of statically watched resources per controller and GVK",
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
	metrics.Registry.MustRegister(StaticWatchedResourcesTotal)
}

/*
Copyright 2025 The k8s-controller-lib Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//nolint:gochecknoglobals,gochecknoinits
package builder

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// MetricNameConversionErrors is the Prometheus metric name for tracking conversion errors.
	MetricNameConversionErrors = "builder_conversion_errors_total"
	// MetricLabelNameController is the label name representing the controller name in Prometheus metrics.
	MetricLabelNameController = "controller"
	// MetricLabelNameAPIVersion is the Prometheus label for resource API version (e.g., "apps/v1", "v1").
	MetricLabelNameAPIVersion = "api_version"
	// MetricLabelNameKind is the Prometheus label for resource kind (e.g., "Deployment", "ConfigMap").
	MetricLabelNameKind = "kind"
	// MetricLabelNameReason is the label name for the error reason in Prometheus metrics.
	MetricLabelNameReason = "reason"
)

const (
	// ReasonTypeNotInScheme indicates the type is not registered in the scheme.
	ReasonTypeNotInScheme = "type_not_in_scheme"
	// ReasonNotClientObject indicates the target type does not implement client.Object.
	ReasonNotClientObject = "not_client_object"
	// ReasonConversionFailed indicates the conversion from unstructured failed.
	ReasonConversionFailed = "conversion_failed"
)

var (
	// ConversionErrorsTotal is a Prometheus counter tracking conversion errors during event processing.
	// Labels: controller, api_version, kind, reason.
	ConversionErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricNameConversionErrors,
			Help: "Total conversion errors during event processing",
		},
		[]string{
			MetricLabelNameController,
			MetricLabelNameAPIVersion,
			MetricLabelNameKind,
			MetricLabelNameReason,
		},
	)
)

func init() {
	metrics.Registry.MustRegister(ConversionErrorsTotal)
}

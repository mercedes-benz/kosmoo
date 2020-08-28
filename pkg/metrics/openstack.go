// SPDX-License-Identifier: MIT

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type openstackMetrics struct {
	duration *prometheus.HistogramVec
	total    *prometheus.CounterVec
	errors   *prometheus.CounterVec
}

var (
	requestMetrics *openstackMetrics
)

func registerOpenStackMetrics() {
	requestMetrics = &openstackMetrics{
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: generateName("openstack_api_request_duration_seconds"),
				Help: "Latency of an OpenStack API call",
			}, []string{"request"}),
		total: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: generateName("openstack_api_requests_total"),
				Help: "Total number of OpenStack API calls",
			}, []string{"request"}),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: generateName("openstack_api_request_errors_total"),
				Help: "Total number of errors for an OpenStack API call",
			}, []string{"request"}),
	}

	prometheus.MustRegister(requestMetrics.duration)
	prometheus.MustRegister(requestMetrics.total)
	prometheus.MustRegister(requestMetrics.errors)
}

// openStackMetric indicates the context for OpenStack metrics.
type openStackMetric struct {
	start      time.Time
	attributes []string
}

// newOpenStackMetric creates a new MetricContext.
func newOpenStackMetric(resource string, request string) *openStackMetric {
	return &openStackMetric{
		start:      time.Now(),
		attributes: []string{resource + "_" + request},
	}
}

// Observe records the request latency and counts the errors.
func (mc *openStackMetric) Observe(err error) error {
	requestMetrics.duration.WithLabelValues(mc.attributes...).Observe(
		time.Since(mc.start).Seconds())
	requestMetrics.total.WithLabelValues(mc.attributes...).Inc()
	if err != nil {
		requestMetrics.errors.WithLabelValues(mc.attributes...).Inc()
	}
	return err
}

package providerproxy

import (
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	metrics struct {
		TotalRequests    *prometheus.CounterVec
		RequestsDuration prometheus.ObserverVec
	}

	RequestMetrics struct {
		Provider   string
		RpcMethod  string
		StatusCode int
		Duration   time.Duration
	}
)

func (m *ProviderProxy) newMetrics() *metrics {
	commonLabels := prometheus.Labels{
		"pipelineName": m.Name(),
		"kind":         Kind,
	}
	prometheusLabels := []string{
		"pipelineName", "kind", "policy", "statusCode", "provider", "rpcMethod",
	}

	return &metrics{
		TotalRequests: prometheushelper.NewCounter(
			"providerproxy_total_requests",
			"the total count of http requests", prometheusLabels).MustCurryWith(commonLabels),
		RequestsDuration: prometheushelper.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "providerproxy_requests_duration",
				Help:    "request processing duration histogram of a backend",
				Buckets: prometheushelper.DefaultDurationBuckets(),
			}, prometheusLabels).MustCurryWith(commonLabels),
	}
}

func (m *ProviderProxy) collectMetrics(requestMetrics RequestMetrics) {
	labels := prometheus.Labels{
		"policy":     m.spec.Policy,
		"statusCode": strconv.Itoa(requestMetrics.StatusCode),
		"provider":   requestMetrics.Provider,
		"rpcMethod":  requestMetrics.RpcMethod,
	}

	m.metrics.TotalRequests.With(labels).Inc()
	m.metrics.RequestsDuration.With(labels).Observe(float64(requestMetrics.Duration.Milliseconds()))
}

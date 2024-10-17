package providerproxy

import (
	"net/http"
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
)

func (m *ProviderProxy) newMetrics() *metrics {

	commonLabels := prometheus.Labels{
		"pipelineName": m.Name(),
		"kind":         Kind,
		"clusterName":  m.spec.Super().Options().ClusterName,
		"clusterRole":  m.spec.Super().Options().ClusterRole,
		"instanceName": m.spec.Super().Options().Name,
	}
	prometheusLabels := []string{
		"clusterName", "clusterRole", "instanceName", "pipelineName", "kind",
		"policy", "statusCode", "provider",
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

type RequestStat struct {
	StatusCode int // e.g. 200
	Duration   time.Duration
	Method     *string // rpc provider method e.g. eth_blockNumber
}

func (m *ProviderProxy) collectMetrics(providerUrl string, response *http.Response) {
	labels := prometheus.Labels{
		"policy":     m.spec.Policy,
		"statusCode": strconv.Itoa(response.StatusCode),
		"provider":   providerUrl,
	}

	m.metrics.TotalRequests.With(labels).Inc()
}

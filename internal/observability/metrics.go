package observability

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	LLMRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "docs_organiser_request_latency_seconds",
		Help:    "Duration of LLM requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"model", "type"})

	LLMTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "docs_organiser_token_usage_total",
		Help: "Total number of tokens used",
	}, []string{"model", "type"}) // type: prompt, completion, total

	TruncationEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "docs_organiser_truncation_events_total",
		Help: "Total number of truncation events",
	}, []string{"model", "strategy"})

	ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "docs_organiser_errors_total",
		Help: "Total number of errors encountered",
	}, []string{"type"}) // type: parsing, timeout, connection, move
)

func StartMetricsServer(port int) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	return server.ListenAndServe()
}

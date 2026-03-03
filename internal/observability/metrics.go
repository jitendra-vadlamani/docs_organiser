package observability

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

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
	}, []string{"type"}) // type: parsing, timeout, connection, move, extraction

	ActiveWorkersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "docs_organiser_active_workers_count",
		Help: "Current number of active pipeline workers",
	})

	MemoryAllocBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "docs_organiser_memory_alloc_bytes",
		Help: "Current bytes allocated and still in use (runtime.MemStats.Alloc)",
	})

	MemorySysBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "docs_organiser_memory_sys_bytes",
		Help: "Total bytes of memory obtained from the OS (runtime.MemStats.Sys)",
	})
)

func StartMetricsServer(port int) error {
	// Start background memory monitor
	go monitorMemory()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	return server.ListenAndServe()
}

func monitorMemory() {
	var m runtime.MemStats
	for {
		runtime.ReadMemStats(&m)
		MemoryAllocBytes.Set(float64(m.Alloc))
		MemorySysBytes.Set(float64(m.Sys))
		time.Sleep(5 * time.Second)
	}
}

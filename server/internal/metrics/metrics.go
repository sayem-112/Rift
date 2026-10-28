// Package metrics defines Rift's Prometheus metrics and the internal
// endpoint that serves them alongside pprof.
package metrics

import (
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var Registry = prometheus.NewRegistry()

var (
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rift_http_requests_total",
		Help: "HTTP requests by route pattern, method and status class.",
	}, []string{"route", "method", "status"})

	HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rift_http_request_duration_seconds",
		Help:    "HTTP request latency by route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"route", "method"})

	JobsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rift_jobs_processed_total",
		Help: "Background jobs by kind and outcome (done, retry, failed).",
	}, []string{"kind", "outcome"})

	JobDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rift_job_duration_seconds",
		Help:    "Background job run time by kind.",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})

	JobsUnfinished = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "rift_jobs_unfinished",
		Help: "Jobs not yet done, by kind and state (pending, running, failed).",
	}, []string{"kind", "state"})

	AuthEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rift_auth_events_total",
		Help: "Sign-ins, refreshes, replayed-token detections and rate-limit refusals.",
	}, []string{"event"})

	Uploads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rift_uploads_total",
		Help: "Upload outcomes: started, deduplicated, completed, complete_failed, aborted, abandoned.",
	}, []string{"outcome"})

	EventStreams = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "rift_event_streams_open",
		Help: "Open server-sent event streams on this instance.",
	})

	EventsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rift_events_sent_total",
		Help: "Change events pushed over event streams.",
	})
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		HTTPRequests, HTTPDuration, JobsProcessed, JobDuration, JobsUnfinished, AuthEvents,
		Uploads, EventStreams, EventsSent,
	)
}

// InternalHandler serves /metrics and /debug/pprof. Bind it to a private
// address only: profiles expose internals.
func InternalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

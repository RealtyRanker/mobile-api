package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mobileapi_requests_total",
		Help: "Total number of API requests, by route and status code.",
	}, []string{"route", "status"})

	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mobileapi_request_duration_seconds",
		Help:    "Duration of API requests, by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	SignInsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mobileapi_sign_ins_total",
		Help: "Total number of device sign-in attempts, by outcome.",
	}, []string{"outcome"})
)

func init() {
	prometheus.MustRegister(RequestsTotal, RequestDuration, SignInsTotal)
}

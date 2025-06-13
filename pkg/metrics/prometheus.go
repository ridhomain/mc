package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all prometheus metrics
type Metrics struct {
	EmailsProcessed prometheus.Counter
	PayloadsSent    prometheus.Counter
	ProcessingTime  prometheus.Histogram
	ErrorsCount     *prometheus.CounterVec
}

// NewMetrics creates new prometheus metrics
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		EmailsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "emails_processed_total",
			Help:      "The total number of processed emails",
		}),
		PayloadsSent: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "payloads_sent_total",
			Help:      "The total number of payloads sent to WhatsApp",
		}),
		ProcessingTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "email_processing_time_seconds",
			Help:      "Time taken to process emails",
			Buckets:   prometheus.DefBuckets,
		}),
		ErrorsCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "errors_total",
			Help:      "The total number of errors",
		}, []string{"operation"}),
	}
}
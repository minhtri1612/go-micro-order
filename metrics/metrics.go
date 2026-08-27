package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orders_created_total",
		Help: "The total number of created orders",
	})

	OrdersUpdated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orders_updated_total",
		Help: "The total number of updated orders",
	})

	OrderStatusUpdated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "order_status_updates_total",
		Help: "The total number of order status updates by status",
	}, []string{"status"})

	OrderProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_processing_duration_seconds",
		Help:    "Time taken to process orders",
		Buckets: prometheus.DefBuckets,
	})

	ActiveOrders = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_orders",
		Help: "The current number of active orders",
	})
)

package bitsgo

import "time"

type MetricsService interface {
	SendTimingMetric(name string, duration time.Duration)
	SendGaugeMetric(name string, value int64)
	SendCounterMetric(name string, value int64)
}

package common

import "github.com/prometheus/prometheus/model/labels"

// StoredMetric the type of the queue
type StoredMetric struct {
	L labels.Labels
	T int64
	V float64
}

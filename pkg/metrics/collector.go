//go:build !aix

package metrics

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	collectCPU  bool
	collectMem  bool
	collectDisk bool
	logger      log.Logger
}

func NewCollector(logger log.Logger) *Collector {
	return &Collector{
		collectCPU: true,
		collectMem: true,
		logger:     logger,
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {

}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {

}

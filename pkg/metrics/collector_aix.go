package metrics

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thorhs/aix_libperfstat/generated"
)

var aixCPUSeconds = prometheus.NewDesc("aix_cpu_ticks_total",
	"cpu seconds running by name",
	[]string{"cpu", "mode"},
	nil)

var aixMemInuse = prometheus.NewDesc("aix_mem_inuse_bytes",
	"memory inuse",
	nil,
	nil)

var aixMemFree = prometheus.NewDesc("aix_mem_free_bytes",
	"memory free",
	nil,
	nil)

var aixDiskFree = prometheus.NewDesc("aix_disk_free_mb",
	"memory free",
	[]string{"vg"},
	nil)

type Collector struct {
	collectCPU  bool
	collectMem  bool
	collectDisk bool
	logger      log.Logger
}

func NewCollector(logger log.Logger) *Collector {
	return &Collector{
		collectCPU:  true,
		collectMem:  true,
		collectDisk: true,
		logger:      logger,
	}
}

func (c *Collector) Describe(descs chan<- *prometheus.Desc) {
	descs <- aixCPUSeconds
	descs <- aixMemInuse
	descs <- aixMemFree
	descs <- aixDiskFree
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	level.Info(c.logger).Log("msg", "collecting cpu")
	if c.collectCPU {
		cpuStats := generated.CollectCpus()
		for k, v := range cpuStats {
			level.Info(c.logger).Log("k", k)
			metrics <- prometheus.MustNewConstMetric(aixCPUSeconds, prometheus.CounterValue, v.User, k, "user")
			metrics <- prometheus.MustNewConstMetric(aixCPUSeconds, prometheus.CounterValue, v.Idle, k, "idle")
			metrics <- prometheus.MustNewConstMetric(aixCPUSeconds, prometheus.CounterValue, v.Wait, k, "wait")
			metrics <- prometheus.MustNewConstMetric(aixCPUSeconds, prometheus.CounterValue, v.Sys, k, "sys")
		}
	}

	if c.collectMem {
		memStats := generated.CollectMemory()
		metrics <- prometheus.MustNewConstMetric(aixMemInuse, prometheus.CounterValue, memStats.RealInuse)
		metrics <- prometheus.MustNewConstMetric(aixMemFree, prometheus.CounterValue, memStats.RealFree)
	}

	if c.collectDisk {
		diskStats := generated.CollectDisks()
		for k, v := range diskStats {
			metrics <- prometheus.MustNewConstMetric(aixDiskFree, prometheus.CounterValue, v.Free, k)
		}
	}
}

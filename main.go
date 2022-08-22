package main

import (
	"context"
	"flag"
	"github.com/mattdurham/aix_agent/pkg/common"
	"github.com/mattdurham/aix_agent/pkg/config"
	"github.com/mattdurham/aix_agent/pkg/metrics"
	"github.com/mattdurham/aix_agent/pkg/scrape"
	write2 "github.com/mattdurham/aix_agent/pkg/write"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var aixUp = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "aix_agent_up",
	Help: "Defines if aix is up and running",
}, []string{"up"})

func main() {

	var configFile string
	flag.StringVar(&configFile, "config.file", "", "the location of the config file")
	flag.Parse()
	if configFile == "" {
		panic("config file not defined")
	}
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		panic(err)
	}
	cfg, err := config.ConvertConfig(configBytes)
	if err != nil {
		panic(err)
	}

	w := log.NewSyncWriter(os.Stdout)
	logger := log.NewLogfmtLogger(w)
	var option level.Option
	switch cfg.LogLevel {
	case "debug":
		option = level.AllowDebug()
	case "info":
		option = level.AllowInfo()
	case "error":
		option = level.AllowError()
	case "warn":
		option = level.AllowWarn()
	default:
		option = level.AllowError()
	}
	logger = level.NewFilter(logger, option)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	r := mux.NewRouter()
	agentMetricsReg := prometheus.NewRegistry()
	agentMetricsReg.MustRegister(aixUp)
	statsReg := prometheus.NewRegistry()

	c := metrics.NewCollector(logger)
	statsReg.MustRegister(c)
	statsReg.MustRegister(aixUp)

	r.Handle("/agent/metrics", promhttp.HandlerFor(agentMetricsReg, promhttp.HandlerOpts{}))
	r.Handle("/metrics", promhttp.HandlerFor(statsReg, promhttp.HandlerOpts{}))
	http.Handle("/", r)

	// setup mem chan
	sendTo := make(chan []*common.StoredMetric)

	// setup scraper
	scraper := scrape.NewScraper(logger, sendTo, "localhost:8000")

	// setup remote write
	write := write2.NewRemote(logger, cfg, sendTo)

	ctx := context.Background()
	go write.Run(ctx)
	go scraper.Run("http://localhost:8000/metrics", 10*time.Second, ctx)

	srv := &http.Server{
		Handler:     r,
		Addr:        "127.0.0.1:8000",
		ReadTimeout: 30 * time.Second,
	}
	aixUp.WithLabelValues("true").Inc()
	level.Info(logger).Log("msg", "serving")
	srv.ListenAndServe()
}

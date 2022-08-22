package scrape

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mattdurham/aix_agent/pkg/common"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/model/timestamp"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type Scraper struct {
	logger log.Logger
	target string
	sendTo chan []*common.StoredMetric
}

func NewScraper(logger log.Logger, sendTo chan []*common.StoredMetric, target string) *Scraper {
	return &Scraper{
		logger: logger,
		target: target,
		sendTo: sendTo,
	}
}

func (s *Scraper) Run(target string, scrapeInterval time.Duration, ctx context.Context) error {
	tick := time.NewTicker(scrapeInterval)
	for {
		select {
		case <-tick.C:
			resp, err := http.Get(target)
			if err != nil {
				level.Error(s.logger).Log("msg", "failure getting from logger", "err", err)
				continue
			}
			content, _ := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			p, err := textparse.New(content, "")
			if err != nil {
				level.Error(s.logger).Log("msg", "error getting parser", "err", err)
				continue
			}
			s.handleRead(p)
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Scraper) handleRead(p textparse.Parser) {
	ms := make([]*common.StoredMetric, 0)
	var defTime = timestamp.FromTime(time.Now())
	for {
		var et textparse.Entry

		et, err := p.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		switch et {
		case textparse.EntryType:
			continue
		case textparse.EntryHelp:
			continue
		case textparse.EntryUnit:
			continue
		case textparse.EntryComment:
			continue
		default:
		}
		lset := make(labels.Labels, 0)
		p.Metric(&lset)
		lset = append(lset, labels.Label{
			Name:  "job_name",
			Value: "aix_exporter",
		})
		_, _, v := p.Series()
		ms = append(ms, &common.StoredMetric{
			L: lset,
			T: defTime,
			V: v,
		})
	}
	s.sendTo <- ms
}

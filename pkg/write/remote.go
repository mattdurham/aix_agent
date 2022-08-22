package write

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/mattdurham/aix_agent/pkg/common"
	"github.com/mattdurham/aix_agent/pkg/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Remote struct {
	mut    sync.Mutex
	logger log.Logger
	client *Client
	memory chan []*common.StoredMetric
	queue  *metricQueue
	cfg    *config.Config
}

func NewRemote(logger log.Logger, cfg *config.Config, memory chan []*common.StoredMetric) *Remote {
	pUrl, _ := url.Parse(cfg.RemoteURL)
	r := &Remote{
		logger: logger,
		memory: memory,
		queue:  new(),
		client: &Client{
			client:           http.DefaultClient,
			url:              pUrl,
			timeout:          20 * time.Second,
			retryOnRateLimit: true,
		},
		cfg: cfg,
	}
	return r
}

func (r *Remote) Run(ctx context.Context) {

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case v := <-r.memory:
				r.queue.Enqueue(v)
			}
		}
	}()
	tick := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-tick.C:
			v := r.queue.Peek()
			if v == nil {
				continue
			}
			pBuf := proto.NewBuffer(nil)
			metrics := StoredMetricToPendingData(v)
			writeReq, _, err := buildWriteRequest(metrics, nil, pBuf, make([]byte, 0))
			if err != nil {
				level.Error(r.logger).Log("msg", "failed to build write request", "err", err)
				continue
			}
			err = r.client.Store(ctx, writeReq, r.cfg.RemoteUser, r.cfg.RemotePassword)
			if err != nil {
				_, ok := err.(RecoverableError)
				if !ok {
					level.Error(r.logger).Log("msg", "failed to send write request", "err", err)
					continue
				}
				level.Error(r.logger).Log("msg", "failed to send write request but retrying", "err", err)

				// Sleep for 5 seconds before trying again
				time.Sleep(5 * time.Second)
				continue
			}
			r.queue.Dequeue()
		case <-ctx.Done():
			return
		}
	}
}

func StoredMetricToPendingData(sm []*common.StoredMetric) []prompb.TimeSeries {
	pendingData := make([]prompb.TimeSeries, len(sm))

	for i, m := range sm {
		converted := labelsToLabelsProto(m.L, make([]prompb.Label, len(m.L)))
		pendingData[i].Labels = converted
		pendingData[i].Samples = append(pendingData[i].Samples, prompb.Sample{
			Value:     m.V,
			Timestamp: m.T,
		})
	}
	return pendingData
}

// labelsToLabelsProto transforms labels into prompb labels. The buffer slice
// will be used to avoid allocations if it is big enough to store the labels.
func labelsToLabelsProto(labels labels.Labels, buf []prompb.Label) []prompb.Label {
	result := buf[:0]
	if cap(buf) < len(labels) {
		result = make([]prompb.Label, 0, len(labels))
	}
	for _, l := range labels {
		result = append(result, prompb.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}
	return result
}

func buildWriteRequest(samples []prompb.TimeSeries, metadata []prompb.MetricMetadata, pBuf *proto.Buffer, buf []byte) ([]byte, int64, error) {
	var highest int64
	for _, ts := range samples {
		// At the moment we only ever append a TimeSeries with a single sample or exemplar in it.
		if len(ts.Samples) > 0 && ts.Samples[0].Timestamp > highest {
			highest = ts.Samples[0].Timestamp
		}
		if len(ts.Exemplars) > 0 && ts.Exemplars[0].Timestamp > highest {
			highest = ts.Exemplars[0].Timestamp
		}
	}

	req := &prompb.WriteRequest{
		Timeseries: samples,
		Metadata:   metadata,
	}

	if pBuf == nil {
		pBuf = proto.NewBuffer(nil) // For convenience in tests. Not efficient.
	} else {
		pBuf.Reset()
	}
	err := pBuf.Marshal(req)
	if err != nil {
		return nil, highest, err
	}

	// snappy uses len() to see if it needs to allocate a new slice. Make the
	// buffer as long as possible.
	if buf != nil {
		buf = buf[0:cap(buf)]
	}
	compressed := snappy.Encode(buf, pBuf.Bytes())
	return compressed, highest, nil
}

type Metrics []*common.StoredMetric

// metricQueue the queue of Metrics
type metricQueue struct {
	items []Metrics
	lock  sync.RWMutex
}

// New creates a new metricQueue
func new() *metricQueue {
	s := &metricQueue{
		items: make([]Metrics, 0),
	}
	return s
}

// Enqueue adds an StoredMetric to the end of the queue
func (s *metricQueue) Enqueue(t Metrics) {
	s.lock.Lock()
	s.items = append(s.items, t)
	s.lock.Unlock()
}

// Dequeue removes an StoredMetric from the start of the queue
func (s *metricQueue) Dequeue() Metrics {
	s.lock.Lock()
	item := s.items[0]
	s.items = s.items[1:len(s.items)]
	s.lock.Unlock()
	return item
}

// Peek gets an StoredMetric from the start of the queue
func (s *metricQueue) Peek() Metrics {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.items) == 0 {
		return nil
	}
	item := s.items[0]
	return item
}

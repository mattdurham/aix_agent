package write

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/prometheus/common/model"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	client           *http.Client
	url              *url.URL
	timeout          time.Duration
	retryOnRateLimit bool
}

type RecoverableError struct {
	error
	retryAfter model.Duration
}

const maxErrMsgLen = 1024
const defaultBackoff = 0

// Store sends a batch of samples to the HTTP endpoint, the request is the proto marshalled
// and encoded bytes from codec.go.
func (c *Client) Store(ctx context.Context, req []byte, username string, password string) error {
	httpReq, err := http.NewRequest("POST", c.url.String(), bytes.NewReader(req))
	if err != nil {
		// Errors from NewRequest are from unparsable URLs, so are not
		// recoverable.
		return err
	}
	httpReq.SetBasicAuth(username, password)
	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("User-Agent", "aix_agent")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	httpResp, err := c.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		// Errors from Client.Do are from (for example) network errors, so are
		// recoverable.
		return RecoverableError{err, defaultBackoff}
	}
	defer func() {
		io.Copy(io.Discard, httpResp.Body)
		httpResp.Body.Close()
	}()

	if httpResp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(httpResp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, line)
	}
	if httpResp.StatusCode/100 == 5 {
		return RecoverableError{err, defaultBackoff}
	}
	if c.retryOnRateLimit && httpResp.StatusCode == http.StatusTooManyRequests {
		return RecoverableError{err, retryAfterDuration(httpResp.Header.Get("Retry-After"))}
	}
	return err
}

// retryAfterDuration returns the duration for the Retry-After header. In case of any errors, it
// returns the defaultBackoff as if the header was never supplied.
func retryAfterDuration(t string) model.Duration {
	parsedDuration, err := time.Parse(http.TimeFormat, t)
	if err == nil {
		s := time.Until(parsedDuration).Seconds()
		return model.Duration(s) * model.Duration(time.Second)
	}
	// The duration can be in seconds.
	d, err := strconv.Atoi(t)
	if err != nil {
		return defaultBackoff
	}
	return model.Duration(d) * model.Duration(time.Second)
}

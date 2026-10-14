// Package overpass runs Overpass QL queries and builds them from simple filters.
package overpass

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paulmach/osm"
)

type Client struct {
	URL  string
	HTTP *http.Client
	// Backoff is the wait before the first retry, doubled on each later one.
	// Zero means the default; tests shorten it.
	Backoff time.Duration
}

const attempts = 3

// Result is either parsed map data or, for outputs such as [out:csv], the raw body.
type Result struct {
	Data *osm.OSM
	Raw  []byte
}

// Query retries on 429 and 504: the public instance answers those whenever its
// per-IP slots are busy, and they clear within seconds.
func (c *Client) Query(ctx context.Context, ql string) (*Result, error) {
	wait := c.Backoff
	if wait == 0 {
		wait = 3 * time.Second
	}
	for i := 1; ; i++ {
		res, err := c.do(ctx, ql)
		var se *statusErr
		busy := errors.As(err, &se) && (se.code == http.StatusTooManyRequests || se.code == http.StatusGatewayTimeout)
		if !busy || i == attempts {
			return res, err
		}
		select {
		case <-time.After(wait):
			wait *= 2
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) do(ctx context.Context, ql string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(url.Values{"data": {ql}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp.StatusCode, body)
	}

	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch ct {
	case "application/json":
		var o osm.OSM
		if err := json.Unmarshal(body, &o); err != nil {
			return nil, fmt.Errorf("decode overpass json: %w", err)
		}
		if remark := remarkOf(body); remark != "" {
			return nil, fmt.Errorf("overpass: %s", remark)
		}
		return &Result{Data: &o}, nil
	case "application/osm3s+xml", "application/xml", "text/xml":
		var o osm.OSM
		if err := xml.Unmarshal(body, &o); err != nil {
			return nil, fmt.Errorf("decode overpass xml: %w", err)
		}
		return &Result{Data: &o}, nil
	default:
		return &Result{Raw: body}, nil
	}
}

// remarkOf surfaces runtime errors (timeouts, memory limits) that Overpass
// reports inside a 200 response instead of as an HTTP error.
func remarkOf(body []byte) string {
	var r struct {
		Remark string `json:"remark"`
	}
	_ = json.Unmarshal(body, &r)
	if strings.Contains(r.Remark, "error") {
		return r.Remark
	}
	return ""
}

type statusErr struct {
	code int
	msg  string
}

func (e *statusErr) Error() string { return e.msg }

func statusError(code int, body []byte) error {
	msg := fmt.Sprintf("overpass: HTTP %d", code)
	switch code {
	case http.StatusTooManyRequests:
		msg = "overpass: rate limited (429); wait a minute or use another instance with OSMCTL_OVERPASS_URL"
	case http.StatusGatewayTimeout:
		msg = "overpass: server busy or query too heavy (504); narrow the area or retry later"
	case http.StatusBadRequest:
		msg = "overpass: query rejected:\n" + errorLines(body)
	}
	return &statusErr{code: code, msg: msg}
}

// errorLines pulls the human-readable messages out of Overpass's HTML error page.
func errorLines(body []byte) string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if i := strings.Index(line, "<strong"); i >= 0 {
			text := line[i:]
			text = strings.NewReplacer("<strong style=\"color:#FF0000\">", "", "</strong>", "", "</p>", "").Replace(text)
			out = append(out, "  "+strings.TrimSpace(htmlUnescape(text)))
		}
	}
	if len(out) == 0 {
		return "  " + strings.TrimSpace(string(body))
	}
	return strings.Join(out, "\n")
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&quot;", `"`, "&lt;", "<", "&gt;", ">", "&amp;", "&").Replace(s)
}

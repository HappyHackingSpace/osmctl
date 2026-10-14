package overpass

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueryRetriesBusyServer(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		if r.FormValue("data") != "node(1);out;" {
			t.Errorf("query not sent as form data: %q", r.FormValue("data"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"elements":[{"type":"node","id":1,"lat":1,"lon":2,"tags":{"name":"x"}}]}`)
	}))
	defer srv.Close()

	c := &Client{URL: srv.URL, HTTP: srv.Client(), Backoff: time.Millisecond}
	res, err := c.Query(context.Background(), "node(1);out;")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || len(res.Data.Nodes) != 1 || res.Data.Nodes[0].Tags.Find("name") != "x" {
		t.Errorf("calls=%d result=%+v", calls.Load(), res.Data)
	}
}

func TestQueryDoesNotRetryBadQuery(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "<p><strong style=\"color:#FF0000\">Error</strong>: line 1: parse error: Unknown type &quot;nod&quot; </p>\n")
	}))
	defer srv.Close()

	c := &Client{URL: srv.URL, HTTP: srv.Client(), Backoff: time.Millisecond}
	_, err := c.Query(context.Background(), "nod(1);out;")
	if err == nil || !strings.Contains(err.Error(), `Unknown type "nod"`) {
		t.Errorf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("bad query was retried %d times", calls.Load())
	}
}

func TestQueryRuntimeRemark(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"elements":[],"remark":"runtime error: Query timed out in \"query\" at line 3 after 2 seconds."}`)
	}))
	defer srv.Close()

	_, err := (&Client{URL: srv.URL, HTTP: srv.Client()}).Query(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v", err)
	}
}

func TestQueryRawOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = io.WriteString(w, "@id\tname\n1\tx\n")
	}))
	defer srv.Close()

	res, err := (&Client{URL: srv.URL, HTTP: srv.Client()}).Query(context.Background(), "[out:csv(::id,name)];node(1);out;")
	if err != nil || string(res.Raw) != "@id\tname\n1\tx\n" {
		t.Errorf("res=%+v err=%v", res, err)
	}
}

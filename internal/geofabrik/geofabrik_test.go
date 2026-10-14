package geofabrik

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegionsAndDownload(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = io.WriteString(w, `{"features":[
				{"properties":{"id":"turkey","parent":"europe","name":"Turkey","urls":{"pbf":"`+srvURL+`/turkey.pbf"}}},
				{"properties":{"id":"iraq","parent":"asia","name":"Iraq","urls":{"pbf":"`+srvURL+`/iraq.pbf"}}}]}`)
		case "/turkey.pbf":
			_, _ = io.WriteString(w, "PBFDATA")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	c := &Client{HTTP: srv.Client(), Index: srv.URL + "/index.json"}
	regions, err := c.Regions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 2 || regions[0].ID != "iraq" {
		t.Fatalf("regions = %+v", regions)
	}
	if m := Search(regions, "TURK"); len(m) != 1 || m[0].ID != "turkey" {
		t.Errorf("search = %+v", m)
	}
	if _, err := Find(regions, "turk"); err == nil || !strings.Contains(err.Error(), "did you mean turkey") {
		t.Errorf("find suggestion: %v", err)
	}

	r, _ := Find(regions, "turkey")
	var buf bytes.Buffer
	var last int64
	n, err := c.Download(context.Background(), r, &buf, func(done, _ int64) { last = done })
	if err != nil || n != 7 || buf.String() != "PBFDATA" || last != 7 {
		t.Errorf("download n=%d last=%d body=%q err=%v", n, last, buf.String(), err)
	}
}

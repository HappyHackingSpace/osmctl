// Package geofabrik finds and downloads regional OSM extracts from
// download.geofabrik.de, the usual starting point for working offline.
package geofabrik

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const IndexURL = "https://download.geofabrik.de/index-v1-nogeom.json"

type Region struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
	Name   string `json:"name"`
	PBF    string `json:"pbf"`
}

type Client struct {
	HTTP  *http.Client
	Index string
}

func (c *Client) Regions(ctx context.Context) ([]Region, error) {
	resp, err := c.get(ctx, c.Index)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var idx struct {
		Features []struct {
			Properties struct {
				ID     string            `json:"id"`
				Parent string            `json:"parent"`
				Name   string            `json:"name"`
				URLs   map[string]string `json:"urls"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("read geofabrik index: %w", err)
	}
	out := make([]Region, 0, len(idx.Features))
	for _, f := range idx.Features {
		p := f.Properties
		out = append(out, Region{ID: p.ID, Parent: p.Parent, Name: p.Name, PBF: p.URLs["pbf"]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Search matches the query against region ids and names, case-insensitively.
func Search(regions []Region, query string) []Region {
	q := strings.ToLower(query)
	var out []Region
	for _, r := range regions {
		if strings.Contains(strings.ToLower(r.ID), q) || strings.Contains(strings.ToLower(r.Name), q) {
			out = append(out, r)
		}
	}
	return out
}

func Find(regions []Region, id string) (Region, error) {
	for _, r := range regions {
		if r.ID == id {
			return r, nil
		}
	}
	if m := Search(regions, id); len(m) > 0 {
		ids := make([]string, 0, min(len(m), 5))
		for _, r := range m[:min(len(m), 5)] {
			ids = append(ids, r.ID)
		}
		return Region{}, fmt.Errorf("no region %q; did you mean %s?", id, strings.Join(ids, ", "))
	}
	return Region{}, fmt.Errorf("no region %q (see `osmctl data regions`)", id)
}

// Download streams the region's PBF into w, reporting progress as it goes.
// total is -1 when the server does not announce a size.
func (c *Client) Download(ctx context.Context, r Region, w io.Writer, progress func(done, total int64)) (int64, error) {
	resp, err := c.get(ctx, r.PBF)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	pw := &progressWriter{w: w, total: resp.ContentLength, fn: progress}
	return io.Copy(pw, resp.Body)
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

type progressWriter struct {
	w           io.Writer
	done, total int64
	fn          func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.fn != nil {
		p.fn(p.done, p.total)
	}
	return n, err
}

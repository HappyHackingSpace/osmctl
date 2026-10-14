// Package nominatim geocodes place names and reverse geocodes coordinates.
package nominatim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/paulmach/osm"
)

type Client struct {
	URL  string
	HTTP *http.Client
}

type Place struct {
	PlaceID     int64             `json:"place_id"`
	OSMType     string            `json:"osm_type"`
	OSMID       int64             `json:"osm_id"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	BoundingBox []string          `json:"boundingbox"`
	Address     map[string]string `json:"address,omitempty"`
}

// FeatureID returns the OSM element behind the place, if it has one.
func (p Place) FeatureID() (osm.FeatureID, bool) {
	id, err := osm.Type(p.OSMType).FeatureID(p.OSMID)
	return id, err == nil && p.OSMID > 0
}

type SearchOptions struct {
	Limit     int
	Countries []string
	Language  string
}

func (c *Client) Search(ctx context.Context, q string, opts SearchOptions) ([]Place, error) {
	v := url.Values{"q": {q}, "format": {"jsonv2"}, "addressdetails": {"1"}}
	if opts.Limit > 0 {
		v.Set("limit", strconv.Itoa(opts.Limit))
	}
	if len(opts.Countries) > 0 {
		v.Set("countrycodes", strings.Join(opts.Countries, ","))
	}
	var places []Place
	if err := c.get(ctx, "/search", v, opts.Language, &places); err != nil {
		return nil, err
	}
	return places, nil
}

// Reverse finds the place at a point. Zoom sets the detail level, from 3
// (country) to 18 (building).
func (c *Client) Reverse(ctx context.Context, lat, lon float64, zoom int, language string) (*Place, error) {
	v := url.Values{
		"lat":            {strconv.FormatFloat(lat, 'f', -1, 64)},
		"lon":            {strconv.FormatFloat(lon, 'f', -1, 64)},
		"format":         {"jsonv2"},
		"addressdetails": {"1"},
	}
	if zoom > 0 {
		v.Set("zoom", strconv.Itoa(zoom))
	}
	var resp struct {
		Place
		Error string `json:"error"`
	}
	if err := c.get(ctx, "/reverse", v, language, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("nominatim: %s", resp.Error)
	}
	return &resp.Place, nil
}

func (c *Client) get(ctx context.Context, path string, v url.Values, language string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.URL, "/")+path+"?"+v.Encode(), nil)
	if err != nil {
		return err
	}
	if language != "" {
		req.Header.Set("Accept-Language", language)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("nominatim: blocked or rate limited (%d); the public instance allows 1 request per second", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("nominatim: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

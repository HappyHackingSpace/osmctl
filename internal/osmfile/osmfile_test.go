package osmfile

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

const sample = "testdata/sample.osm"

var ctx = context.Background()

func nodeIDs(d *osm.Data) []int64 {
	var ids []int64
	for _, n := range d.Nodes {
		ids = append(ids, n.ID)
	}
	slices.Sort(ids)
	return ids
}

func TestInfo(t *testing.T) {
	st, err := Info(ctx, sample)
	if err != nil {
		t.Fatal(err)
	}
	if st.Nodes != 4 || st.Ways != 2 || st.Relations != 1 {
		t.Errorf("counts = %d/%d/%d", st.Nodes, st.Ways, st.Relations)
	}
	if st.Bounds.MinLat != 37.91 || st.Bounds.MaxLon != 41 {
		t.Errorf("bounds = %+v", st.Bounds)
	}
	if st.FirstEdit.Year() != 2020 || st.LastEdit.Year() != 2025 {
		t.Errorf("edits %v .. %v", st.FirstEdit, st.LastEdit)
	}
}

func TestExtractByTagIncludesWayGeometry(t *testing.T) {
	fs, _ := tagfilter.ParseAll([]string{"highway=footway"})
	o, err := Extract(ctx, sample, Query{Filters: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Ways) != 1 || o.Ways[0].ID != 10 {
		t.Fatalf("ways = %v", o.Ways)
	}
	if got := nodeIDs(o); !slices.Equal(got, []int64{2, 3}) {
		t.Errorf("geometry nodes = %v, want [2 3]", got)
	}
	for _, n := range o.Nodes {
		if len(n.Tags) != 0 {
			t.Errorf("geometry node %d kept its tags and would look like a match", n.ID)
		}
	}
}

func TestExtractNodesOnly(t *testing.T) {
	fs, _ := tagfilter.ParseAll([]string{"amenity=cafe"})
	o, err := Extract(ctx, sample, Query{Filters: fs, Types: []osm.Type{osm.TypeNode}})
	if err != nil {
		t.Fatal(err)
	}
	if got := nodeIDs(o); !slices.Equal(got, []int64{1, 4}) || len(o.Ways)+len(o.Relations) != 0 {
		t.Errorf("nodes=%v ways=%d relations=%d", got, len(o.Ways), len(o.Relations))
	}
}

func TestExtractBBox(t *testing.T) {
	b := &osm.Bounds{MinLat: 37.9, MaxLat: 37.92, MinLon: 40.2, MaxLon: 40.25}
	o, err := Extract(ctx, sample, Query{Bounds: b})
	if err != nil {
		t.Fatal(err)
	}
	// Nodes 1 and 2 are inside; way 10 crosses the edge, so node 3 comes along
	// with its tags; way 11 and node 4 are entirely outside.
	if got := nodeIDs(o); !slices.Equal(got, []int64{1, 2, 3}) {
		t.Errorf("nodes = %v", got)
	}
	if len(o.Ways) != 1 || len(o.Relations) != 1 {
		t.Errorf("ways=%v relations=%d", o.Ways, len(o.Relations))
	}
	for _, n := range o.Nodes {
		if n.ID == 3 && n.Tags.Find("barrier") != "gate" {
			t.Error("a plain area extract must keep tags on boundary nodes")
		}
	}
}

func TestOpenCompressedAndUnknown(t *testing.T) {
	raw, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	gzPath := filepath.Join(t.TempDir(), "sample.osm.gz")
	f, _ := os.Create(gzPath)
	zw := gzip.NewWriter(f)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	_ = f.Close()

	st, err := Info(ctx, gzPath)
	if err != nil || st.Nodes != 4 {
		t.Errorf("gzip: %+v, %v", st, err)
	}
	if _, err := Info(ctx, "map.geojson"); err == nil {
		t.Error("expected an error for an unsupported extension")
	}
}

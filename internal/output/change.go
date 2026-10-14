package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/paulmach/osm"
)

// Change renders an osmChange. JSON keeps the create/modify/delete split;
// GeoJSON has nowhere to put it, so it falls back to JSON as well.
func Change(w io.Writer, c *osm.Change, f Format) error {
	return structured(w, f, c, func(tw *tabwriter.Writer) {
		empty := true
		for _, sec := range []struct {
			name string
			o    *osm.OSM
		}{{"create", c.Create}, {"modify", c.Modify}, {"delete", c.Delete}} {
			for _, id := range featureIDs(sec.o) {
				empty = false
				fmt.Fprintf(tw, "%s\t%s\n", sec.name, id)
			}
		}
		if empty {
			fmt.Fprintln(tw, "no changes")
		}
	})
}

func featureIDs(o *osm.OSM) osm.FeatureIDs {
	if o == nil {
		return nil
	}
	return o.FeatureIDs()
}

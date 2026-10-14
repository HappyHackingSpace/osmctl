package output

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/happyhackingspace/osmctl/internal/nominatim"
	"github.com/happyhackingspace/osmctl/internal/osm"
)

func Places(w io.Writer, ps []nominatim.Place, f Format) error {
	switch f {
	case GeoJSON:
		fc := &osm.FeatureCollection{Type: "FeatureCollection", Features: []*osm.Feature{}}
		for _, p := range ps {
			lat, _ := strconv.ParseFloat(p.Lat, 64)
			lon, _ := strconv.ParseFloat(p.Lon, 64)
			feat := &osm.Feature{Type: "Feature", Geometry: osm.Geometry{Type: "Point", Coordinates: [2]float64{lon, lat}},
				Properties: map[string]string{"name": p.Name, "display_name": p.DisplayName, "category": p.Category, "type": p.Type}}
			if id, ok := p.ElementID(); ok {
				feat.ID = id.String()
			}
			fc.Features = append(fc.Features, feat)
		}
		return WriteJSON(w, fc)
	case XML:
		return fmt.Errorf("xml output is not available for geocoding results")
	case JSON:
		return WriteJSON(w, ps)
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if len(ps) == 0 {
		fmt.Fprintln(tw, "no places found")
		return tw.Flush()
	}
	fmt.Fprintln(tw, "ELEMENT\tTYPE\tLOCATION\tNAME")
	for _, p := range ps {
		el := "-"
		if id, ok := p.ElementID(); ok {
			el = id.String()
		}
		fmt.Fprintf(tw, "%s\t%s=%s\t%s,%s\t%s\n", el, p.Category, p.Type, p.Lat, p.Lon, oneLine(p.DisplayName, 80))
	}
	return tw.Flush()
}

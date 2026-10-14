package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"

	"github.com/happyhackingspace/osmctl/internal/nominatim"
)

func Places(w io.Writer, ps []nominatim.Place, f Format) error {
	switch f {
	case GeoJSON:
		fc := geojson.NewFeatureCollection()
		for _, p := range ps {
			lat, _ := strconv.ParseFloat(p.Lat, 64)
			lon, _ := strconv.ParseFloat(p.Lon, 64)
			feat := geojson.NewFeature(orb.Point{lon, lat})
			raw, _ := json.Marshal(p)
			_ = json.Unmarshal(raw, &feat.Properties)
			fc.Append(feat)
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
		if id, ok := p.FeatureID(); ok {
			el = id.String()
		}
		fmt.Fprintf(tw, "%s\t%s=%s\t%s,%s\t%s\n", el, p.Category, p.Type, p.Lat, p.Lon, oneLine(p.DisplayName, 80))
	}
	return tw.Flush()
}

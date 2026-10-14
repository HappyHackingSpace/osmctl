package validate

import (
	"fmt"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/stage"
)

// replacements maps deprecated tags to what the community uses instead: a
// short list of the most common cases from iD's deprecated.json.
var replacements = []struct {
	key, value string
	use        string
}{
	{"amenity", "ev_charging", "amenity=charging_station"},
	{"amenity", "public_building", "building=public plus the tag for what it is"},
	{"amenity", "swimming_pool", "leisure=swimming_pool"},
	{"barrier", "wire_fence", "barrier=fence + fence_type=chain_link"},
	{"barrier", "wood_fence", "barrier=fence + fence_type=wood"},
	{"highway", "abandoned", "abandoned:highway=* (or remove the way if it is gone)"},
	{"highway", "ford", "ford=yes on the node"},
	{"landuse", "farm", "landuse=farmland or landuse=farmyard"},
	{"leisure", "beach", "natural=beach"},
	{"natural", "marsh", "natural=wetland + wetland=marsh"},
	{"shop", "fish", "shop=seafood"},
	{"shop", "organic", "shop=supermarket + organic=only"},
	{"sport", "gaelic_football", "sport=gaelic_games"},
	{"power", "sub_station", "power=substation"},
	{"waterway", "riverbank", "natural=water + water=river"},
}

func deprecated(s *stage.Stage) []Issue {
	var out []Issue
	for _, el := range staged(s) {
		ts := osm.TagsOf(el)
		id := el.ElementID().String()
		for _, r := range replacements {
			if ts.Find(r.key) == r.value {
				out = append(out, Issue{Warning, id, "deprecated-tag", fmt.Sprintf("%s=%s is deprecated; use %s", r.key, r.value, r.use)})
			}
		}
		if ts.Find("highway") == "road" {
			out = append(out, Issue{Warning, id, "vague-tag", "highway=road means the road type is unknown; pick residential, track, service, unclassified, ..."})
		}
	}
	return out
}

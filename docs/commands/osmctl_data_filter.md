## osmctl data filter

Extract features by tag and/or area from a file

### Synopsis

Extract features from a local file by tag (same filters as `osmctl find`),
by bounding box, or both. Ways come with every node they need, so the
result converts cleanly:

  -o xml      an .osm file for JOSM or other tools
  -o geojson  for QGIS, uMap, Leaflet and friends
  -o json     OSM JSON
  -o text     a table of what matched (default)

With only --bbox, everything touching the box is extracted.

```
osmctl data filter <file> [filter]... [--bbox ...] [flags]
```

### Examples

```
  osmctl data filter turkey-latest.osm.pbf amenity=drinking_water --bbox 40.1,37.8,40.3,38.0
  osmctl data filter region.osm.pbf highway=bus_stop -o geojson > stops.geojson
  osmctl data extract region.osm.pbf --bbox 40.20,37.90,40.25,37.93 -o xml > old-city.osm
```

### Options

```
      --bbox string    only features touching minlon,minlat,maxlon,maxlat
  -h, --help           help for filter
  -t, --type strings   limit to node, way and/or relation
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl data](osmctl_data.md)	 - Work with local .osm and .osm.pbf files


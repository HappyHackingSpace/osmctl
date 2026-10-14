## osmctl map

Download all map data in a small area

### Synopsis

Download every node, way and relation in a bounding box from the OSM API.

The API caps this at 50,000 nodes and 0.25 square degrees. For larger areas
use `osmctl query` (Overpass) or a regional extract with `osmctl data`.

```
osmctl map --bbox <minlon,minlat,maxlon,maxlat> [flags]
```

### Examples

```
  osmctl map --bbox 40.2290,37.9100,40.2330,37.9130 -o xml > area.osm
```

### Options

```
      --bbox string   bounding box as minlon,minlat,maxlon,maxlat
  -h, --help          help for map
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


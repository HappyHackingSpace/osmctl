## osmctl reverse

Find the address at a point (Nominatim)

```
osmctl reverse <lat,lon> [flags]
```

### Examples

```
  osmctl reverse 37.9144,40.2306
  osmctl reverse 37.9144,40.2306 --zoom 10
```

### Options

```
  -h, --help          help for reverse
  -l, --lang string   preferred result language, e.g. ku, tr, en
  -z, --zoom int      detail level from 3 (country) to 18 (building) (default 18)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


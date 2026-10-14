## osmctl query

Run an Overpass QL query

### Synopsis

Run an Overpass QL query, given as an argument, with --file, or on stdin.

JSON and XML responses are parsed, so -o text, json, xml and geojson all
work. For GeoJSON, ways and relations need their nodes in the result: end
the query with "out body; >; out skel qt;". Other outputs, such as
[out:csv], are printed as Overpass returns them.

For simple tag searches, `osmctl find` writes the query for you.

```
osmctl query [overpass-ql] [flags]
```

### Examples

```
  osmctl query '[out:json];node["amenity"="cafe"](around:500,37.91,40.23);out;'
  osmctl query -f stations.overpassql -o geojson > stations.geojson
  echo 'node(1);out;' | osmctl query
```

### Options

```
  -f, --file string   read the query from a file
  -h, --help          help for query
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


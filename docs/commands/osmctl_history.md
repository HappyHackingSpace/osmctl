## osmctl history

Show every version of an element

```
osmctl history <element> [flags]
```

### Examples

```
  osmctl history node/2187164523
  osmctl history w42 -o json | jq '.elements[].user'
```

### Options

```
  -h, --help   help for history
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


## osmctl reset

Unstage changes

```
osmctl reset (<element>... | --all) [flags]
```

### Examples

```
  osmctl reset node/2187164523
  osmctl reset --all
```

### Options

```
      --all    unstage everything
  -h, --help   help for reset
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


## osmctl diff

Show staged changes in detail

### Synopsis

Show what every staged change does: tags added (+), removed (-) and
changed (~), nodes moved, elements created and deleted.

-o xml prints the osmChange that push uploads.

```
osmctl diff [flags]
```

### Options

```
  -h, --help   help for diff
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


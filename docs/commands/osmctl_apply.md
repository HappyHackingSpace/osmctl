## osmctl apply

Stage an osmChange file made by another tool

### Synopsis

Stage every change in an osmChange (.osc) file, for example one written by
a script or exported from JOSM. Use - to read standard input.

Modified and deleted elements must carry the version they were based on,
so the server can detect conflicts.

```
osmctl apply <file.osc | -> [flags]
```

### Examples

```
  osmctl apply fixes.osc
  python fix_names.py | osmctl apply -
```

### Options

```
  -h, --help   help for apply
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


## osmctl browse

Open an element, changeset or note on the website

```
osmctl browse <element|changeset/ID|note/ID> [flags]
```

### Examples

```
  osmctl browse w42
  osmctl browse changeset/151234567 --print
```

### Options

```
  -h, --help    help for browse
  -p, --print   print the URL instead of opening it
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


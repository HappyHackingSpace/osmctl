## osmctl move

Move a node to a new location

```
osmctl move <node> --to <lat,lon> [flags]
```

### Examples

```
  osmctl move node/2187164523 --to 37.91452,40.23071
```

### Options

```
  -h, --help        help for move
      --to string   new location as lat,lon
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


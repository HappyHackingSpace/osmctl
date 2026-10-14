## osmctl add

Create new nodes and ways

### Synopsis

Stage new elements. They get negative placeholder ids (n-1, w-2, ...)
that you can use in later commands; the server assigns real ids on push.

### Options

```
  -h, --help   help for add
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal
* [osmctl add node](osmctl_add_node.md)	 - Add a point, such as a shop, bench or tree
* [osmctl add way](osmctl_add_way.md)	 - Add a line or area through existing or staged nodes


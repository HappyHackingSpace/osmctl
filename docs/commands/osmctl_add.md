## osmctl add

Create new nodes, ways and relations

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
* [osmctl add node](osmctl_add_node.md)	 - Add a point, or a node on existing ways to connect to them
* [osmctl add relation](osmctl_add_relation.md)	 - Add a relation, such as a route or a multipolygon
* [osmctl add way](osmctl_add_way.md)	 - Add a line or area through nodes, or from a GPS track


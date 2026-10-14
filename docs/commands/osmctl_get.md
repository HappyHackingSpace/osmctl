## osmctl get

Show nodes, ways and relations

### Synopsis

Show the current state of one or more elements.

Elements can be written as node/123, n123, w42, r7, or pasted as an
openstreetmap.org URL.

```
osmctl get <element>... [flags]
```

### Examples

```
  osmctl get node/2187164523
  osmctl get w42 --full -o geojson > way.geojson
  osmctl get https://www.openstreetmap.org/relation/7 -o json
  osmctl get n123 --version 2
```

### Options

```
      --full          include every node and member needed to draw ways and relations
  -h, --help          help for get
      --version int   show a specific historical version
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


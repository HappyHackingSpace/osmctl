## osmctl add way

Add a line or area through existing or staged nodes

```
osmctl add way --nodes <n1,n2,...> [key=value]... [flags]
```

### Examples

```
  osmctl add node --at 37.91,40.23
  osmctl add node --at 37.92,40.24 --on way/123456
  osmctl add way --nodes n-1,n-2 highway=footway
```

### Options

```
      --close           close the way into a ring (for areas)
  -h, --help            help for way
      --nodes strings   node ids in order, e.g. n-1,n-2,n123
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes and ways


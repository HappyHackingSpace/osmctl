## osmctl add node

Add a point, such as a shop, bench or tree

```
osmctl add node --at <lat,lon> [key=value]... [flags]
```

### Examples

```
  osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes
```

### Options

```
      --at string   location as lat,lon
  -h, --help        help for node
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes and ways


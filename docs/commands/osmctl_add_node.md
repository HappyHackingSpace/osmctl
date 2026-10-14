## osmctl add node

Add a point, or a node on existing ways to connect to them

### Synopsis

Add a node at a location.

With --on, the node is placed onto the nearest point of that way instead,
so new roads can be joined to it. Repeat --on to make a junction shared by
several ways. If a node of the way is already within a metre, that node is
used instead of adding a new one next to it.

```
osmctl add node --at <lat,lon> [--on <way>]... [key=value]... [flags]
```

### Examples

```
  osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes
  osmctl add node --at 38.43412,40.28429 --on way/362907199
  osmctl add node --at 38.4401,40.2853 --on w1568227783 --on w362907199
```

### Options

```
      --at string    location as lat,lon
  -h, --help         help for node
      --on strings   place the node onto this way (repeatable)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes, ways and relations


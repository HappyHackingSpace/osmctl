## osmctl add way

Add a line or area through nodes, or from a GPS track

### Synopsis

Add a way through existing or staged nodes, or from a recorded GPS track.

With --from-gpx, the track is simplified (Douglas-Peucker, --simplify metres)
so GPS jitter does not become hundreds of nodes. --start-on and --end-on
place the first and last node onto existing ways, so the new road is
connected to the network.

```
osmctl add way (--nodes <n1,n2,...> | --from-gpx <file>) [key=value]... [flags]
```

### Examples

```
  osmctl add node --at 37.91,40.23
  osmctl add node --at 37.92,40.24 --on way/123456
  osmctl add way --nodes n-1,n-2 highway=footway
  osmctl add way --from-gpx drive.gpx --start-on way/362907199 highway=track surface=unpaved
```

### Options

```
      --close             close the way into a ring (for areas)
      --end-on string     with --from-gpx: place the last node onto this way
      --from-gpx string   build the way from a GPX track
  -h, --help              help for way
      --nodes strings     node ids in order, e.g. n-1,n-2,n123
      --simplify float    with --from-gpx: drop points within this many metres of the line (default 3)
      --start-on string   with --from-gpx: place the first node onto this way
      --track int         which track or route in the GPX file (1 is the first) (default 1)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes, ways and relations


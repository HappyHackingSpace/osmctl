## osmctl member add

Add members to a relation

```
osmctl member add <relation> <element[:role]>... [flags]
```

### Examples

```
  osmctl member add r1234 way/5678:outer
  osmctl member add r1234 n-1:stop --at 0
```

### Options

```
      --at int   insert at this position (0 is first; default is the end)
  -h, --help     help for add
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl member](osmctl_member.md)	 - Add or remove members of a relation


## osmctl add relation

Add a relation, such as a route or a multipolygon

```
osmctl add relation --member <element[:role]>... [key=value]... [flags]
```

### Examples

```
  osmctl add relation type=route route=hiking name="Pîr Ezîz trail" --member w1568227783 --member w-3
  osmctl add relation type=multipolygon natural=water --member w-1:outer --member w-2:inner
```

### Options

```
  -h, --help             help for relation
      --member strings   member as element or element:role, in order (repeatable)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes, ways and relations


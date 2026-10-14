## osmctl check

Look for mistakes in staged changes

### Synopsis

Check staged changes the way iD and JOSM do before saving.

Errors would break the data and stop `osmctl push`: untagged nodes that belong
to nothing, references to deleted elements, malformed ways.

Warnings deserve a look: roads not connected to the road network, roads
crossing other roads, railways or waterways without a junction, deprecated
or vague tags, and (with -m) a vague changeset comment.

The connection and crossing checks download the map around the edits;
--offline skips them.

```
osmctl check [flags]
```

### Examples

```
  osmctl check
  osmctl check -m "Add footpath to the shrine"
```

### Options

```
  -h, --help             help for check
  -m, --message string   also check this changeset comment
      --offline          skip the checks that download surrounding map data
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


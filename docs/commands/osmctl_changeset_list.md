## osmctl changeset list

List recent changesets, filtered by user, area or state

```
osmctl changeset list [flags]
```

### Examples

```
  osmctl changeset list --user "Some Mapper"
  osmctl changeset list --mine --open
  osmctl changeset list --bbox 40.1,37.8,40.3,38.0 --limit 20
```

### Options

```
      --bbox string   only changesets intersecting minlon,minlat,maxlon,maxlat
      --closed        only closed changesets
  -h, --help          help for list
  -n, --limit int     maximum number of changesets (API maximum is 100) (default 25)
      --mine          only your changesets (requires login)
      --open          only open changesets
  -u, --user string   user display name or numeric id
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl changeset](osmctl_changeset.md)	 - Inspect, list, download and discuss changesets


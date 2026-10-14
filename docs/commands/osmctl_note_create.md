## osmctl note create

Report a problem or missing detail at a location

### Synopsis

Create a map note. Describe what is wrong or missing so that someone on
the ground can fix it. Logged-in notes are attributed to you; without a
login the note is created anonymously.

```
osmctl note create --at <lat,lon> -m <text> [flags]
```

### Examples

```
  osmctl note create --at 37.9144,40.2306 -m "The bakery here closed in 2026; now a pharmacy."
```

### Options

```
      --at string        location as lat,lon
  -h, --help             help for create
  -m, --message string   what is wrong or missing
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl note](osmctl_note.md)	 - Report problems and resolve map notes


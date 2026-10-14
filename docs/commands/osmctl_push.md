## osmctl push

Upload staged changes as one changeset

### Synopsis

Upload every staged change as a single changeset.

The diff and the results of `osmctl check` are shown and you are asked to
confirm first; errors stop the push. The upload is atomic:
if any element changed on the server since you staged it, nothing is
written and your stage is kept so you can resolve the conflict.

Write a comment that says what you changed and why; reviewers read it.
Practice on the sandbox with --server dev before editing the real map.

```
osmctl push -m <comment> [flags]
```

### Examples

```
  osmctl push -m "Add opening hours to cafes on Gazi Caddesi" --source survey
  osmctl --server dev push -m "Testing osmctl" --yes
```

### Options

```
      --hashtags string   hashtags for the changeset, e.g. #hhs;#mapathon
  -h, --help              help for push
  -m, --message string    changeset comment: what changed and why (required)
      --no-check          skip the checks for mistakes (see osmctl check)
      --source string     where the information came from, e.g. survey, local knowledge, Bing
      --tag strings       extra changeset tag as key=value (repeatable)
  -y, --yes               do not ask for confirmation
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal


## osmctl

Read, query, edit and contribute to OpenStreetMap from the terminal

### Synopsis

osmctl is an all-in-one command line tool for OpenStreetMap.

Read elements, history and changesets, query with Overpass, geocode with
Nominatim, report and resolve notes, edit the map through a git-like
stage, diff and push workflow, and work with local .osm and .pbf files.

Reading needs no account. Editing needs `osmctl auth login`.
Practice on the sandbox first: `osmctl --server dev ...`

### Options

```
  -h, --help            help for osmctl
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl add](osmctl_add.md)	 - Create new nodes and ways
* [osmctl apply](osmctl_apply.md)	 - Stage an osmChange file made by another tool
* [osmctl auth](osmctl_auth.md)	 - Log in to OpenStreetMap and manage your token
* [osmctl browse](osmctl_browse.md)	 - Open an element, changeset or note on the website
* [osmctl changeset](osmctl_changeset.md)	 - Inspect, list, download and discuss changesets
* [osmctl data](osmctl_data.md)	 - Work with local .osm and .osm.pbf files
* [osmctl delete](osmctl_delete.md)	 - Delete elements
* [osmctl diff](osmctl_diff.md)	 - Show staged changes in detail
* [osmctl find](osmctl_find.md)	 - Find map features by tag, near a point, in a box or in a named place
* [osmctl geocode](osmctl_geocode.md)	 - Find places by name or address (Nominatim)
* [osmctl get](osmctl_get.md)	 - Show nodes, ways and relations
* [osmctl history](osmctl_history.md)	 - Show every version of an element
* [osmctl map](osmctl_map.md)	 - Download all map data in a small area
* [osmctl move](osmctl_move.md)	 - Move a node to a new location
* [osmctl note](osmctl_note.md)	 - Report problems and resolve map notes
* [osmctl push](osmctl_push.md)	 - Upload staged changes as one changeset
* [osmctl query](osmctl_query.md)	 - Run an Overpass QL query
* [osmctl reset](osmctl_reset.md)	 - Unstage changes
* [osmctl reverse](osmctl_reverse.md)	 - Find the address at a point (Nominatim)
* [osmctl status](osmctl_status.md)	 - List staged changes
* [osmctl tag](osmctl_tag.md)	 - Set or remove tags on an element
* [osmctl user](osmctl_user.md)	 - Show a user's public profile


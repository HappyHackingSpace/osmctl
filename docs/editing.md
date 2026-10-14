# Editing

osmctl edits the map the way git edits code. Changes are **staged** locally, you **review** them, and then you **push** them as one changeset:

```
tag / add / move / delete / apply   ->   status / diff   ->   push
```

Nothing reaches OpenStreetMap until `push`, and `push` asks before it uploads.

## Practice first

The [development server](https://master.apis.dev.openstreetmap.org) is a separate copy of OpenStreetMap for testing. Create an account there, then:

```sh
export OSMCTL_SERVER=dev
osmctl auth login
```

Every command below now works against the sandbox. Unset `OSMCTL_SERVER` when you're ready for the real map. Staged edits are kept per server, so practice edits can never leak into a real push.

## Changing tags

`key=value` sets a tag and `key-` removes one:

```sh
osmctl tag node/2187164523 opening_hours="Mo-Sa 08:00-20:00" wheelchair=yes
osmctl tag w42 name:ku="Kela Amedê" fixme-
```

Quote values that contain spaces. osmctl rejects empty values, keys or values longer than 255 characters, and stray whitespace before anything is uploaded.

Look up tag meanings on the [OSM wiki](https://wiki.openstreetmap.org/wiki/Map_features) or [taginfo](https://taginfo.openstreetmap.org).

## Adding nodes and ways

```sh
osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes
```

New elements get negative **placeholder ids** (`n-1`, `n-2`, `w-3`, ...). Use them in later commands, for example to draw a way through new nodes:

```sh
osmctl add node --at 37.91430,40.23050       # n-1
osmctl add node --at 37.91445,40.23080       # n-2
osmctl add way --nodes n-1,n-2,n2187164523 highway=footway
```

Add `--close` to close the way into a ring for an area, such as a building. On push the server assigns real ids and osmctl prints the mapping.

## Connecting to existing roads

A new road only joins the network if it shares a node with an existing one. Put a node onto the existing way, then start the new way there:

```sh
osmctl add node --at 37.91430,40.23050 --on way/123456     # n-1, snapped onto way/123456
osmctl add node --at 37.91600,40.23300                     # n-2
osmctl add way --nodes n-1,n-2 highway=track
```

`--on` moves the point to the nearest spot on the way and inserts the node there. If one of the way's nodes is already within a metre, that node is used instead. Repeat `--on` to create a junction shared by several ways, for example where a new path meets two roads at once. `osmctl check` warns about roads that are not connected to anything.

## From a GPS track

A track you recorded while driving or walking a road is the best source for its shape. Import it from a GPX file (OsmAnd, Strava, Garmin and most phone apps export GPX):

```sh
osmctl add way --from-gpx drive.gpx --start-on way/362907199 highway=track surface=unpaved
osmctl push -m "Add track to the shrine from GPS" --source GPS
```

- The track is simplified so GPS jitter doesn't turn into hundreds of nodes: points within `--simplify` metres (default 3) of the line are dropped.
- `--start-on` and `--end-on` place the first and last node onto existing ways, so the road is connected.
- `--track N` picks a track when the file has several.
- `osmctl check` warns if the new road runs along one that is already mapped.

## Relations

Relations group elements with roles: routes, multipolygons, turn restrictions.

```sh
osmctl add relation type=route route=hiking name="Pîr Ezîz trail" --member way/1568227783 --member w-3
osmctl member add r1234 way/5678:outer
osmctl member add r1234 n-1:stop --at 0      # insert first; order matters for routes
osmctl member remove r1234 way/5678
```

Members are written as an element, optionally followed by `:role`. `osmctl check` reports relations without members or without a `type` tag.

## Moving and deleting

```sh
osmctl move node/2187164523 --to 37.91452,40.23071
osmctl delete node/2187164523
```

The server refuses to delete a node that a way still uses, or an element that a relation still lists. Delete or edit those in the same push.

## Applying change files

Scripts and other editors can produce osmChange (`.osc`) files. Stage one with:

```sh
osmctl apply fixes.osc
python fix_names.py | osmctl apply -
```

Modified and deleted elements must include the `version` they were based on, which is how the server detects conflicts. Placeholder ids in the file are renumbered so they can't collide with what you have already staged.

## Reviewing

```sh
osmctl status    # what is staged
osmctl diff      # exactly what changes
```

```
+ create node/-1
    at 37.9144000, 40.2306000
  + amenity=bench
  + backrest=yes

~ modify node/2187164523
  ~ opening_hours: Mo-Fr 08:00-18:00 -> Mo-Sa 08:00-20:00
  + wheelchair=yes
  - fixme=check hours
```

`osmctl diff -o xml` prints the osmChange that `push` will upload.

## Checking for mistakes

```sh
osmctl check
```

`check` runs the kind of checks iD and JOSM run before saving. `push` runs them too.

| Kind | Examples | Effect |
|---|---|---|
| error | an untagged new node that belongs to nothing, a way using a node staged for deletion, the same node twice in a row | `push` refuses to upload |
| warning | a road running along an already mapped road, a road not connected to any other road, a road crossing another road, railway or waterway without a shared node, deprecated tags such as `highway=ford`, `highway=road`, a vague changeset comment | shown before you confirm |

The connection and crossing checks download the map around your edits. Roads marked `bridge`, `tunnel` or a different `layer` are not reported as crossings. Use `--offline` to skip the download, or `push --no-check` to skip the checks entirely.

Undo with `reset`:

```sh
osmctl reset node/2187164523
osmctl reset --all
```

Editing an element back to how it was unstages it automatically.

## Pushing

```sh
osmctl push -m "Add opening hours and wheelchair access from survey" --source survey
```

`push` shows the diff, asks for confirmation, then opens a changeset, uploads everything, closes the changeset and prints its URL. The upload is **atomic**: the server applies all of it or none of it.

| Flag | Changeset tag |
|---|---|
| `-m`, `--message` (required) | `comment`: what you changed and why |
| `--source` | `source`: where the information came from (`survey`, `local knowledge`, an imagery name) |
| `--hashtags` | `hashtags`: for mapathons and campaigns, e.g. `#hhs;#mapathon` |
| `--tag key=value` | any other changeset tag |

In scripts, `--yes` skips the confirmation. Without a terminal, osmctl refuses to push unless `--yes` is given.

### Large uploads

A changeset holds at most 10,000 changes on openstreetmap.org (osmctl asks the server for its limit). Larger stages are split automatically into several changesets, uploaded in an order the server can apply: new nodes, then ways, then relations, then modifications, then deletions. Each comment gets "(part 2 of 3)" appended. If a part fails, the parts before it are already on the map and only the rest stays staged; run `push` again to continue.

Uploads this large are usually imports or automated edits, which need community agreement first.

### Conflicts

If someone else changed an element after you staged it, the server rejects the whole upload with a version conflict. Nothing is written and your stage is kept. Look at the newer version, reset your copy and redo the edit:

```sh
osmctl get node/2187164523
osmctl reset node/2187164523
osmctl tag node/2187164523 wheelchair=yes
osmctl push -m "..."
```

## Good practice

- **One topic per changeset.** "Add bus stops on Gazi Caddesi" is easy to review; "various fixes" is not.
- **Only map what you know.** Survey, local knowledge and permitted imagery are good sources. Copying from Google Maps or other copyrighted sources is not allowed.
- **Large, scripted or imported edits need discussion first.** Read the [Automated Edits code of conduct](https://wiki.openstreetmap.org/wiki/Automated_Edits_code_of_conduct) and the [Import guidelines](https://wiki.openstreetmap.org/wiki/Import/Guidelines). osmctl warns before pushing 100 or more changes.
- **Answer your changeset discussions.** Other mappers may ask questions; see them with `osmctl changeset list --mine` and `osmctl changeset view <id>`.

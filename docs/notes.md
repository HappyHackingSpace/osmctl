# Notes

Map notes let anyone report a problem, or something missing, at a location, without editing the map. Local mappers then fix the map and close the note. They are a good way to contribute when you know something is wrong but not exactly how to map it.

## Reporting

```sh
osmctl note create --at 37.9144,40.2306 -m "The bakery here closed in 2026; it is a pharmacy now."
```

Write for someone who has never been there: what is wrong, what is true now, and how you know. If you are not logged in, the note is created anonymously.

## Browsing

```sh
osmctl note list --near 37.9144,40.2306 --radius 2000
osmctl note list --bbox 40.1,37.8,40.3,38.0 --all          # include closed notes
osmctl note list --text "bus stop"                          # search note text
osmctl note view 4123456
```

By default only open notes are listed. `--recently-closed` adds notes closed in the last 7 days, and `--all` adds every closed note.

## Resolving

These need a login:

```sh
osmctl note comment 4123456 -m "Is this the shop on the corner, or the one next to it?"
osmctl note close 4123456 -m "Fixed in changeset 151234567, thanks!"
osmctl note reopen 4123456 -m "Still not right: the entrance is on the other street."
```

A typical loop when you're fixing notes in your area:

```sh
osmctl note list --near 37.9144,40.2306 --radius 3000
osmctl note view 4123456
osmctl tag node/... shop=chemist name="..."
osmctl push -m "Bakery replaced by pharmacy, from note 4123456" --source survey
osmctl note close 4123456 -m "Fixed, thanks for reporting!"
```

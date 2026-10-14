# Security policy

## Reporting a vulnerability

Please report security problems privately through [GitHub's security advisories](https://github.com/happyhackingspace/osmctl/security/advisories/new), not in a public issue. We aim to respond within a week.

Useful to include: the osmctl version (`osmctl --version`), what an attacker can do, and steps to reproduce.

## Scope

Relevant reports include anything that could leak or misuse an OpenStreetMap access token, make osmctl upload changes the user did not review, or run code from untrusted input such as `.osm`, `.pbf`, `.gpx` or `.osc` files.

Never include a real access token in a report. If you think yours was exposed, revoke it under **My Settings > OAuth 2 applications** on openstreetmap.org, or with `osmctl auth logout`.

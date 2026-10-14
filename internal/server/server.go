// Package server describes the OpenStreetMap instances osmctl can talk to.
package server

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

type Server struct {
	Name      string
	API       string
	Web       string
	Overpass  string
	Nominatim string
	// ClientID identifies osmctl as an OAuth 2.0 application on this instance.
	// OAuth applications are registered per instance, so each server has its own.
	ClientID string
}

const (
	Production = "osm"
	Dev        = "dev"
)

var known = map[string]Server{
	Production: {
		Name:      Production,
		API:       "https://api.openstreetmap.org/api/0.6",
		Web:       "https://www.openstreetmap.org",
		Overpass:  "https://overpass-api.de/api/interpreter",
		Nominatim: "https://nominatim.openstreetmap.org",
	},
	Dev: {
		Name: Dev,
		API:  "https://master.apis.dev.openstreetmap.org/api/0.6",
		Web:  "https://master.apis.dev.openstreetmap.org",
		// The dev instance has no Overpass or Nominatim of its own; the public ones
		// still answer read questions about the real map.
		Overpass:  "https://overpass-api.de/api/interpreter",
		Nominatim: "https://nominatim.openstreetmap.org",
	},
}

func Names() []string {
	names := make([]string, 0, len(known))
	for n := range known {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Lookup resolves a server by name or by the web URL of a self-hosted instance,
// then applies OSMCTL_* environment overrides.
func Lookup(nameOrURL string) (Server, error) {
	s, ok := known[nameOrURL]
	if !ok {
		u, err := url.Parse(nameOrURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return Server{}, fmt.Errorf("unknown server %q (use %s, or a URL like https://osm.example.org)", nameOrURL, strings.Join(Names(), ", "))
		}
		web := strings.TrimRight(u.String(), "/")
		s = Server{
			Name:      u.Host,
			API:       web + "/api/0.6",
			Web:       web,
			Overpass:  known[Production].Overpass,
			Nominatim: known[Production].Nominatim,
		}
	}
	return withEnv(s), nil
}

func withEnv(s Server) Server {
	for env, field := range map[string]*string{
		"OSMCTL_API_URL":       &s.API,
		"OSMCTL_OVERPASS_URL":  &s.Overpass,
		"OSMCTL_NOMINATIM_URL": &s.Nominatim,
		"OSMCTL_CLIENT_ID":     &s.ClientID,
	} {
		if v := os.Getenv(env); v != "" {
			*field = v
		}
	}
	return s
}

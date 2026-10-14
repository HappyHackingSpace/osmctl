package server

import "testing"

func TestLookup(t *testing.T) {
	s, err := Lookup(Dev)
	if err != nil || s.API != "https://master.apis.dev.openstreetmap.org/api/0.6" {
		t.Fatalf("Lookup(dev) = %+v, %v", s, err)
	}

	s, err = Lookup("https://osm.example.org/")
	if err != nil {
		t.Fatal(err)
	}
	if s.API != "https://osm.example.org/api/0.6" || s.Name != "osm.example.org" {
		t.Errorf("custom server = %+v", s)
	}

	if _, err := Lookup("nope"); err == nil {
		t.Error("expected error for unknown server")
	}
}

func TestLookupEnvOverride(t *testing.T) {
	t.Setenv("OSMCTL_OVERPASS_URL", "https://overpass.example.org/api/interpreter")
	s, err := Lookup(Production)
	if err != nil {
		t.Fatal(err)
	}
	if s.Overpass != "https://overpass.example.org/api/interpreter" {
		t.Errorf("override ignored: %s", s.Overpass)
	}
}

func TestProductionHasBuiltinClientID(t *testing.T) {
	t.Setenv("OSMCTL_CLIENT_ID", "")
	s, err := Lookup(Production)
	if err != nil || s.ClientID == "" {
		t.Fatalf("production must work without OSMCTL_CLIENT_ID: %+v, %v", s, err)
	}
	t.Setenv("OSMCTL_CLIENT_ID", "override")
	if s, _ := Lookup(Production); s.ClientID != "override" {
		t.Errorf("OSMCTL_CLIENT_ID should override the built-in id, got %q", s.ClientID)
	}
}

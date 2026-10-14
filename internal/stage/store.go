package stage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DirEnv overrides where stages are kept.
const DirEnv = "OSMCTL_STAGE_DIR"

// Path is the stage file for a server. One file per server keeps sandbox
// practice from ever leaking into a production push.
func Path(server string) (string, error) {
	dir := os.Getenv(DirEnv)
	if dir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cfg, "osmctl", "stage")
	}
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid server URL %q", server)
	}
	name := strings.NewReplacer(":", "_", "/", "_").Replace(u.Host)
	return filepath.Join(dir, name+".json"), nil
}

// Load returns the stage for server, or an empty one.
func Load(server string) (*Stage, error) {
	p, err := Path(server)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return New(server), nil
	}
	if err != nil {
		return nil, err
	}
	var s Stage
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("read stage %s: %w", p, err)
	}
	if s.Server != server {
		return nil, fmt.Errorf("stage %s belongs to %s, not %s", p, s.Server, server)
	}
	return &s, nil
}

// Save writes atomically so an interrupted command never corrupts the stage.
func (s *Stage) Save() error {
	p, err := Path(s.Server)
	if err != nil {
		return err
	}
	if s.Empty() {
		err := os.Remove(p)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".stage-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// Package tagfilter parses the tag conditions users type on the command line
// and evaluates them, locally or as Overpass QL.
package tagfilter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/paulmach/osm"
)

type Filter struct {
	Key, Op, Value string
	re             *regexp.Regexp
}

var syntax = regexp.MustCompile(`^(!?)([^=!~]+)(?:(=|!=|~|!~)(.*))?$`)

// Parse accepts key, !key, key=value, key!=value, key~regex and key!~regex.
func Parse(s string) (Filter, error) {
	m := syntax.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || strings.TrimSpace(m[2]) == "" {
		return Filter{}, fmt.Errorf("invalid tag filter %q (expected key, !key, key=value, key!=value, key~regex)", s)
	}
	f := Filter{Key: strings.TrimSpace(m[2]), Op: m[3], Value: m[4]}
	if m[1] == "!" {
		if f.Op != "" {
			return Filter{}, fmt.Errorf("invalid tag filter %q: use key!=value instead of !key=value", s)
		}
		f.Op = "!"
	}
	if f.Op != "" && f.Op != "!" && f.Value == "" {
		return Filter{}, fmt.Errorf("invalid tag filter %q: missing value", s)
	}
	if f.Op == "~" || f.Op == "!~" {
		re, err := regexp.Compile(f.Value)
		if err != nil {
			return Filter{}, fmt.Errorf("invalid tag filter %q: %w", s, err)
		}
		f.re = re
	}
	return f, nil
}

func ParseAll(args []string) ([]Filter, error) {
	out := make([]Filter, 0, len(args))
	for _, a := range args {
		f, err := Parse(a)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// Match follows Overpass semantics: != and !~ also match when the key is absent.
func (f Filter) Match(tags osm.Tags) bool {
	v, ok := tags.Map()[f.Key], tags.HasTag(f.Key)
	switch f.Op {
	case "":
		return ok
	case "!":
		return !ok
	case "=":
		return ok && v == f.Value
	case "!=":
		return !ok || v != f.Value
	case "~":
		return ok && f.re.MatchString(v)
	case "!~":
		return !ok || !f.re.MatchString(v)
	}
	return false
}

func MatchAll(fs []Filter, tags osm.Tags) bool {
	for _, f := range fs {
		if !f.Match(tags) {
			return false
		}
	}
	return true
}

// QL renders the filter as an Overpass QL condition.
func (f Filter) QL() string {
	k := quote(f.Key)
	switch f.Op {
	case "":
		return "[" + k + "]"
	case "!":
		return "[!" + k + "]"
	default:
		return "[" + k + f.Op + quote(f.Value) + "]"
	}
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

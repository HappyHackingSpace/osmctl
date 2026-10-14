// Package validate catches mistakes in staged edits before they are uploaded,
// the checks iD and JOSM run before saving.
package validate

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/stage"
)

type Severity string

const (
	// Error blocks the upload: the data would be broken.
	Error Severity = "error"
	// Warning is worth a look but may be intended.
	Warning Severity = "warning"
	// Notice reports a check that could not run.
	Notice Severity = "notice"
)

type Issue struct {
	Severity Severity `json:"severity"`
	Element  string   `json:"element,omitempty"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
}

// MapSource provides the existing data around the edits, for checks that
// need the neighbourhood (connectivity, crossings).
type MapSource interface {
	Map(ctx context.Context, b *osm.Bounds) (*osm.Data, error)
}

// Check runs every check over the stage. src may be nil to skip the checks
// that need surrounding map data.
func Check(ctx context.Context, s *stage.Stage, src MapSource) []Issue {
	var issues []Issue
	issues = append(issues, references(s)...)
	issues = append(issues, orphans(s)...)
	issues = append(issues, wayShape(s)...)
	issues = append(issues, deprecated(s)...)
	if src != nil {
		issues = append(issues, network(ctx, s, src)...)
	}
	sort.SliceStable(issues, func(i, j int) bool { return rank(issues[i].Severity) < rank(issues[j].Severity) })
	return issues
}

func rank(s Severity) int {
	switch s {
	case Error:
		return 0
	case Warning:
		return 1
	}
	return 2
}

func HasErrors(issues []Issue) bool {
	return slices.ContainsFunc(issues, func(i Issue) bool { return i.Severity == Error })
}

// Comment checks a changeset comment, which reviewers read first.
func Comment(comment string) []Issue {
	c := strings.ToLower(strings.TrimSpace(comment))
	vague := map[string]bool{"test": true, "edit": true, "edits": true, "update": true, "updates": true, "fix": true, "fixes": true, "changes": true, "osmctl": true}
	if len([]rune(c)) < 10 || vague[c] {
		return []Issue{{Severity: Warning, Code: "vague-comment",
			Message: fmt.Sprintf("changeset comment %q does not say what changed or why", comment)}}
	}
	return nil
}

func staged(s *stage.Stage) []osm.Element {
	return append(s.Create.Elements(), s.Modify.Elements()...)
}

// references finds staged ways and relations that point at elements staged
// for deletion; the server would reject the upload.
func references(s *stage.Stage) []Issue {
	var out []Issue
	deleted := func(id osm.ID) bool { return s.Delete.Find(id) != nil }
	for _, el := range staged(s) {
		switch v := el.(type) {
		case *osm.Way:
			for _, n := range v.Nodes {
				if id := (osm.ID{Type: osm.TypeNode, Ref: n}); deleted(id) {
					out = append(out, Issue{Error, v.ElementID().String(), "deleted-member", fmt.Sprintf("uses %s, which is staged for deletion", id)})
				}
			}
		case *osm.Relation:
			for _, m := range v.Members {
				if deleted(m.ElementID()) {
					out = append(out, Issue{Error, v.ElementID().String(), "deleted-member", fmt.Sprintf("has member %s, which is staged for deletion", m.ElementID())})
				}
			}
		}
	}
	return out
}

// orphans finds new untagged nodes that no staged way or relation uses: they
// carry no information and are a classic leftover of an aborted drawing.
func orphans(s *stage.Stage) []Issue {
	used := map[int64]bool{}
	for _, el := range staged(s) {
		switch v := el.(type) {
		case *osm.Way:
			for _, n := range v.Nodes {
				used[n] = true
			}
		case *osm.Relation:
			for _, m := range v.Members {
				if m.Type == osm.TypeNode {
					used[m.Ref] = true
				}
			}
		}
	}
	var out []Issue
	if s.Create == nil {
		return nil
	}
	for _, n := range s.Create.Nodes {
		if len(n.Tags) == 0 && !used[n.ID] {
			out = append(out, Issue{Error, n.ElementID().String(), "orphan-node", "new node has no tags and is not part of any way or relation"})
		}
	}
	return out
}

func wayShape(s *stage.Stage) []Issue {
	var out []Issue
	for _, el := range staged(s) {
		w, ok := el.(*osm.Way)
		if !ok {
			continue
		}
		id := w.ElementID().String()
		distinct := map[int64]bool{}
		for _, n := range w.Nodes {
			distinct[n] = true
		}
		if len(distinct) < 2 {
			out = append(out, Issue{Error, id, "short-way", "way needs at least two different nodes"})
		}
		for i := 1; i < len(w.Nodes); i++ {
			if w.Nodes[i] == w.Nodes[i-1] {
				out = append(out, Issue{Error, id, "repeated-node", fmt.Sprintf("node/%d appears twice in a row", w.Nodes[i])})
			}
		}
		if len(osm.TagsOf(w)) == 0 && !inRelation(s, w.ElementID()) {
			out = append(out, Issue{Warning, id, "untagged-way", "way has no tags and is not in any relation, so it means nothing on the map"})
		}
	}
	return out
}

func inRelation(s *stage.Stage, id osm.ID) bool {
	for _, el := range staged(s) {
		if r, ok := el.(*osm.Relation); ok && slices.ContainsFunc(r.Members, func(m osm.Member) bool { return m.ElementID() == id }) {
			return true
		}
	}
	// A modified way may already belong to a relation on the server; only
	// new ways are known for sure to have none.
	return id.Ref > 0
}

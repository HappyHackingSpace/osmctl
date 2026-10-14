package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/stage"
)

var actionMark = map[stage.Action]string{stage.Create: "+", stage.Modify: "~", stage.Delete: "-"}

// Diff renders staged changes like a patch: + created, ~ modified, - deleted.
func Diff(w io.Writer, entries []stage.Entry, f Format) error {
	if f != Text {
		if entries == nil {
			entries = []stage.Entry{}
		}
		return WriteJSON(w, entries)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "nothing staged")
		return err
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s %s %s\n", actionMark[e.Action], e.Action, e.Element)
		for _, o := range e.Other {
			fmt.Fprintf(&b, "    %s\n", o)
		}
		if e.Action == stage.Delete {
			continue
		}
		for _, t := range e.Tags {
			switch {
			case t.Old == "":
				fmt.Fprintf(&b, "  + %s=%s\n", t.Key, t.New)
			case t.New == "":
				fmt.Fprintf(&b, "  - %s=%s\n", t.Key, t.Old)
			default:
				fmt.Fprintf(&b, "  ~ %s: %s -> %s\n", t.Key, t.Old, t.New)
			}
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// Status lists staged elements one per line.
func Status(w io.Writer, entries []stage.Entry, f Format) error {
	if f != Text {
		return Diff(w, entries, f)
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "nothing staged")
		return err
	}
	counts := map[stage.Action]int{}
	var b strings.Builder
	for _, e := range entries {
		counts[e.Action]++
		fmt.Fprintf(&b, "  %s %-7s %s\n", actionMark[e.Action], e.Action, e.Element)
	}
	fmt.Fprintf(&b, "\n%d to create, %d to modify, %d to delete\n", counts[stage.Create], counts[stage.Modify], counts[stage.Delete])
	_, err := io.WriteString(w, b.String())
	return err
}

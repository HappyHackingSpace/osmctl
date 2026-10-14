package output

import (
	"fmt"
	"io"

	"github.com/happyhackingspace/osmctl/internal/validate"
)

var issueMark = map[validate.Severity]string{validate.Error: "error", validate.Warning: "warning", validate.Notice: "note"}

// Issues renders validation results. Text output is meant for people, so it
// ends with a one-line verdict.
func Issues(w io.Writer, issues []validate.Issue, f Format) error {
	if f != Text {
		if issues == nil {
			issues = []validate.Issue{}
		}
		return WriteJSON(w, issues)
	}
	counts := map[validate.Severity]int{}
	for _, i := range issues {
		counts[i.Severity]++
		where := ""
		if i.Element != "" {
			where = i.Element + ": "
		}
		fmt.Fprintf(w, "%-7s %s%s\n", issueMark[i.Severity], where, i.Message)
	}
	switch {
	case counts[validate.Error] > 0:
		_, err := fmt.Fprintf(w, "\n%d error(s), %d warning(s). Fix the errors before pushing.\n", counts[validate.Error], counts[validate.Warning])
		return err
	case counts[validate.Warning] > 0:
		_, err := fmt.Fprintf(w, "\n%d warning(s). Check them before pushing.\n", counts[validate.Warning])
		return err
	}
	_, err := fmt.Fprintln(w, "No problems found.")
	return err
}

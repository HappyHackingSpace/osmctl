package browser

import "testing"

func TestOpenRejectsNonHTTP(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "javascript:alert(1)", "/usr/bin/open", "-a Calculator"} {
		if err := Open(u); err == nil {
			t.Errorf("Open(%q) should be refused", u)
		}
	}
}

package osm

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// EachXML streams the elements of an OSM XML file without loading it whole.
// With nodesOnly it stops at the first way, since files list nodes first.
func EachXML(ctx context.Context, r io.Reader, nodesOnly bool, fn func(Element) error) error {
	dec := xml.NewDecoder(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode osm xml: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		var el Element
		switch start.Name.Local {
		case "node":
			var x xNode
			err = dec.DecodeElement(&x, &start)
			el = x.node()
		case "way", "relation":
			if nodesOnly {
				return nil
			}
			if start.Name.Local == "way" {
				var x xWay
				err = dec.DecodeElement(&x, &start)
				el = x.way()
			} else {
				var x xRelation
				err = dec.DecodeElement(&x, &start)
				el = x.relation()
			}
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("decode osm xml: %w", err)
		}
		if err := fn(el); err != nil {
			return err
		}
	}
}

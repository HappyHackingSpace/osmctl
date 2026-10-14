package osm

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// jElement is OSM JSON, the format of the API's .json endpoints and of
// Overpass [out:json].
type jElement struct {
	Type      Type              `json:"type"`
	ID        int64             `json:"id"`
	Lat       *float64          `json:"lat,omitempty"`
	Lon       *float64          `json:"lon,omitempty"`
	Nodes     []int64           `json:"nodes,omitempty"`
	Members   []Member          `json:"members,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	Version   int               `json:"version,omitempty"`
	Changeset int64             `json:"changeset,omitempty"`
	User      string            `json:"user,omitempty"`
	UID       int64             `json:"uid,omitempty"`
	Timestamp string            `json:"timestamp,omitempty"`
	// Absent means visible, as in XML.
	Visible *bool `json:"visible,omitempty"`
}

type jDoc struct {
	// The API sends the version as a string, Overpass as a number.
	Version   any        `json:"version,omitempty"`
	Generator string     `json:"generator,omitempty"`
	Elements  []jElement `json:"elements"`
	// Overpass reports runtime errors here, inside a 200 response.
	Remark string `json:"remark,omitempty"`
}

func toJ(el Element) jElement {
	m := MetaOf(el)
	j := jElement{Version: m.Version, Changeset: m.Changeset, User: m.User, UID: m.UID, Timestamp: formatTime(m.Timestamp, time.RFC3339)}
	if !m.Visible && m.Version > 0 {
		f := false
		j.Visible = &f
	}
	if ts := TagsOf(el); len(ts) > 0 {
		j.Tags = ts.Map()
	}
	switch v := el.(type) {
	case *Node:
		j.Type, j.ID, j.Lat, j.Lon = TypeNode, v.ID, &v.Lat, &v.Lon
	case *Way:
		j.Type, j.ID, j.Nodes = TypeWay, v.ID, v.Nodes
	case *Relation:
		j.Type, j.ID, j.Members = TypeRelation, v.ID, v.Members
	}
	return j
}

func (j jElement) element() (Element, error) {
	meta := Meta{Version: j.Version, Changeset: j.Changeset, User: j.User, UID: j.UID,
		Visible: j.Visible == nil || *j.Visible, Timestamp: parseTime(j.Timestamp, time.RFC3339)}
	tags := tagsFromMap(j.Tags)
	if len(tags) == 0 {
		tags = nil
	}
	switch j.Type {
	case TypeNode:
		n := &Node{ID: j.ID, Tags: tags, Meta: meta}
		if j.Lat != nil && j.Lon != nil {
			n.Lat, n.Lon = *j.Lat, *j.Lon
		}
		return n, nil
	case TypeWay:
		return &Way{ID: j.ID, Nodes: j.Nodes, Tags: tags, Meta: meta}, nil
	case TypeRelation:
		return &Relation{ID: j.ID, Members: j.Members, Tags: tags, Meta: meta}, nil
	}
	return nil, fmt.Errorf("unsupported element type %q", j.Type)
}

func (d *Data) MarshalJSON() ([]byte, error) {
	doc := jDoc{Version: "0.6", Generator: "osmctl", Elements: []jElement{}}
	for _, el := range d.Elements() {
		doc.Elements = append(doc.Elements, toJ(el))
	}
	return json.Marshal(doc)
}

// UnmarshalJSON reads OSM JSON. Elements of other types, such as Overpass
// areas, are skipped.
func (d *Data) UnmarshalJSON(b []byte) error {
	var doc jDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	*d = Data{}
	for _, j := range doc.Elements {
		el, err := j.element()
		if err != nil {
			continue
		}
		d.Add(el)
	}
	return nil
}

// ReadJSON decodes OSM JSON, reporting an Overpass runtime error if present.
func ReadJSON(r io.Reader) (*Data, string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, "", fmt.Errorf("decode osm json: %w", err)
	}
	var doc struct {
		Remark string `json:"remark"`
	}
	_ = json.Unmarshal(b, &doc)
	return &d, doc.Remark, nil
}

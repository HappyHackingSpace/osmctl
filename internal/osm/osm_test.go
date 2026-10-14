package osm

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseID(t *testing.T) {
	valid := map[string]ID{
		"node/123":                             {TypeNode, 123},
		"n123":                                 {TypeNode, 123},
		"W42":                                  {TypeWay, 42},
		"r7":                                   {TypeRelation, 7},
		"relation 7":                           {TypeRelation, 7},
		" way/9 ":                              {TypeWay, 9},
		"n-1":                                  {TypeNode, -1},
		"way/-3":                               {TypeWay, -3},
		"https://www.openstreetmap.org/way/42": {TypeWay, 42},
		"https://www.openstreetmap.org/node/5#map=19/37.9/40.2": {TypeNode, 5},
	}
	for in, want := range valid {
		got, err := ParseID(in)
		if err != nil || got != want {
			t.Errorf("ParseID(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "123", "x123", "node/", "node/abc", "n0", "https://example.org/", "changeset/1", "n99999999999999999999"} {
		if _, err := ParseID(in); err == nil {
			t.Errorf("ParseID(%q): expected error", in)
		}
	}
	if s := (ID{TypeWay, -3}).String(); s != "way/-3" {
		t.Errorf("String() = %s", s)
	}
}

const apiXML = `<?xml version="1.0" encoding="UTF-8"?>
<osm version="0.6" generator="test">
 <bounds minlat="37.9" minlon="40.2" maxlat="38" maxlon="40.3"/>
 <node id="1" visible="true" version="3" changeset="9" timestamp="2026-01-02T03:04:05Z" user="a" uid="7" lat="37.91" lon="40.23">
  <tag k="amenity" v="cafe"/>
 </node>
 <node id="2" lat="0" lon="0"/>
 <node id="3" visible="false" version="2" lat="0" lon="0"/>
 <way id="10" version="1"><nd ref="1"/><nd ref="2"/><tag k="highway" v="path"/></way>
 <relation id="20" version="1"><member type="way" ref="10" role="outer"/><tag k="type" v="multipolygon"/></relation>
 <changeset id="5" created_at="2026-01-01T00:00:00Z" open="false" user="a" uid="7" changes_count="12" comments_count="1" min_lat="1" min_lon="2" max_lat="3" max_lon="4">
  <tag k="comment" v="hi"/>
  <discussion><comment date="2026-01-03T00:00:00Z" uid="8" user="b"><text>nice</text></comment></discussion>
 </changeset>
 <changeset id="6" open="true" num_changes="4"/>
 <note lon="40.23" lat="37.91">
  <id>99</id><date_created>2026-08-23 17:17:04 UTC</date_created><status>open</status>
  <comments><comment><date>2026-08-23 17:17:04 UTC</date><action>opened</action><text>bench missing</text></comment></comments>
 </note>
 <user id="1" display_name="Steve" account_created="2005-09-13T15:32:57Z"><description>hi</description><changesets count="1184"/><traces count="23"/></user>
</osm>`

func TestReadXML(t *testing.T) {
	d, err := ReadXML(strings.NewReader(apiXML))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Nodes) != 3 || len(d.Ways) != 1 || len(d.Relations) != 1 || d.Bounds.MaxLon != 40.3 {
		t.Fatalf("decoded %+v", d)
	}
	n := d.Nodes[0]
	if n.Version != 3 || n.User != "a" || n.Tags.Find("amenity") != "cafe" || !n.Timestamp.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("node = %+v", n)
	}
	if !d.Nodes[1].Visible {
		t.Error("an element without a visible attribute must be visible")
	}
	if d.Nodes[2].Visible {
		t.Error("visible=false was lost")
	}
	if w := d.Ways[0]; len(w.Nodes) != 2 || w.Nodes[1] != 2 {
		t.Errorf("way = %+v", w)
	}
	if m := d.Relations[0].Members[0]; m.Type != TypeWay || m.Ref != 10 || m.Role != "outer" {
		t.Errorf("member = %+v", m)
	}

	c := d.Changesets[0]
	if c.Changes != 12 || c.Bounds.MaxLon != 4 || c.Tags.Find("comment") != "hi" || len(c.Discussion) != 1 || c.Discussion[0].Text != "nice" {
		t.Errorf("changeset = %+v", c)
	}
	if d.Changesets[1].Changes != 4 || d.Changesets[1].Bounds != nil {
		t.Errorf("legacy num_changes changeset = %+v", d.Changesets[1])
	}
	note := d.Notes[0]
	if note.ID != 99 || note.Status != "open" || note.Created.Year() != 2026 || note.Comments[0].Text != "bench missing" {
		t.Errorf("note = %+v", note)
	}
	if u := d.Users[0]; u.Name != "Steve" || u.Changesets != 1184 || u.Traces != 23 || u.Created.Year() != 2005 {
		t.Errorf("user = %+v", u)
	}
}

func TestXMLRoundTrip(t *testing.T) {
	d, err := ReadXML(strings.NewReader(apiXML))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteXML(&buf, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<osm version="0.6" generator="osmctl">`) {
		t.Errorf("JOSM needs a version on the root:\n%s", buf.String()[:200])
	}
	again, err := ReadXML(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if again.Len() != d.Len() || again.Nodes[2].Visible || !again.Nodes[1].Visible ||
		again.Changesets[0].Changes != 12 || again.Notes[0].Created != d.Notes[0].Created || again.Users[0].Traces != 23 {
		t.Errorf("round trip changed the data:\n%+v\n%+v", d, again)
	}
}

func TestEncodeUpload(t *testing.T) {
	c := &Change{
		Create: &Data{
			Nodes: []*Node{{ID: -1, Lat: 0, Lon: 0, Tags: Tags{{"name", `A & "B"`}}, Meta: Meta{Visible: true}}},
			Ways:  []*Way{{ID: -2, Nodes: []int64{-1, 3}}},
		},
		Modify: &Data{Relations: []*Relation{{ID: 3, Members: []Member{{TypeWay, 8, "outer"}},
			Meta: Meta{Version: 4, User: "x", UID: 9, Timestamp: time.Now()}}}},
		Delete: &Data{Nodes: []*Node{{ID: 1, Lat: 1, Lon: 2, Meta: Meta{Version: 1}}}, Ways: []*Way{{ID: 9, Meta: Meta{Version: 2}}}},
	}
	b, err := EncodeUpload(c, 77)
	if err != nil {
		t.Fatal(err)
	}
	want := `<osmChange version="0.6">` +
		`<create><node id="-1" changeset="77" lat="0" lon="0"><tag k="name" v="A &amp; &#34;B&#34;"></tag></node>` +
		`<way id="-2" changeset="77"><nd ref="-1"></nd><nd ref="3"></nd></way></create>` +
		`<modify><relation id="3" version="4" changeset="77"><member type="way" ref="8" role="outer"></member></relation></modify>` +
		`<delete><way id="9" version="2" changeset="77"></way><node id="1" version="1" changeset="77" lat="1" lon="2"></node></delete>` +
		`</osmChange>`
	if string(b) != want {
		t.Errorf("got\n%s\nwant\n%s", b, want)
	}
}

func TestReadChange(t *testing.T) {
	c, err := ReadChange(strings.NewReader(`<osmChange version="0.6">
		<create><node id="-1" lat="1" lon="2"/></create>
		<modify><way id="5" version="2"><nd ref="1"/></way></modify>
		<create><way id="-2"><nd ref="-1"/></way></create>
		<delete><relation id="9" version="1"/></delete>
	</osmChange>`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Create.Len() != 2 || c.Modify.Len() != 1 || c.Delete.Len() != 1 {
		t.Errorf("sections = %d/%d/%d", c.Create.Len(), c.Modify.Len(), c.Delete.Len())
	}
	if c.Create.Ways[0].Nodes[0] != -1 {
		t.Errorf("placeholder reference lost: %+v", c.Create.Ways[0])
	}
}

func TestJSONRoundTrip(t *testing.T) {
	d, _ := ReadXML(strings.NewReader(apiXML))
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var again Data
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatal(err)
	}
	if again.Len() != d.Len() {
		t.Fatalf("lost elements: %s", b)
	}
	n := again.Nodes[0]
	if n.Version != 3 || n.User != "a" || n.Tags.Find("amenity") != "cafe" || n.Lat != 37.91 || !n.Visible || n.Timestamp != d.Nodes[0].Timestamp {
		t.Errorf("node = %+v", n)
	}
	if again.Nodes[2].Visible {
		t.Error("visible=false lost in JSON")
	}
	if again.Nodes[1].Lat != 0 || again.Nodes[1].Lon != 0 {
		t.Error("a node at 0,0 must keep its coordinates")
	}
	if again.Relations[0].Members[0].Role != "outer" || again.Ways[0].Nodes[1] != 2 {
		t.Errorf("way/relation lost data: %s", b)
	}
}

func TestReadJSONOverpass(t *testing.T) {
	d, remark, err := ReadJSON(strings.NewReader(`{"version":0.6,"elements":[
		{"type":"node","id":1,"lat":1.5,"lon":2.5,"tags":{"name":"x"}},
		{"type":"way","id":2,"nodes":[1,3],"center":{"lat":1,"lon":2}},
		{"type":"area","id":3600000001}
	],"remark":"runtime error: Query timed out"}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Len() != 2 || d.Nodes[0].Tags.Find("name") != "x" || len(d.Ways[0].Nodes) != 2 {
		t.Errorf("data = %+v", d)
	}
	if !strings.Contains(remark, "timed out") {
		t.Errorf("remark = %q", remark)
	}
}

func TestDataFindRemove(t *testing.T) {
	d := &Data{}
	d.Add(&Way{ID: -3})
	d.Add(&Node{ID: -3})
	if d.Find(ID{TypeWay, -3}) == nil || d.Find(ID{TypeNode, -3}) == nil || d.Find(ID{TypeRelation, -3}) != nil {
		t.Error("Find must distinguish types with the same ref")
	}
	d.Remove(ID{TypeWay, -3})
	if d.Find(ID{TypeWay, -3}) != nil || d.Len() != 1 {
		t.Error("Remove failed")
	}
}

func TestEachXML(t *testing.T) {
	var ids []string
	err := EachXML(context.Background(), strings.NewReader(apiXML), false, func(el Element) error {
		ids = append(ids, el.ElementID().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, " ") != "node/1 node/2 node/3 way/10 relation/20" {
		t.Errorf("ids = %v", ids)
	}
	ids = nil
	_ = EachXML(context.Background(), strings.NewReader(apiXML), true, func(el Element) error {
		ids = append(ids, el.ElementID().String())
		return nil
	})
	if len(ids) != 3 {
		t.Errorf("nodesOnly read %v", ids)
	}
}

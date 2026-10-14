package osm

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// pbw is a tiny protobuf writer for building PBF fixtures.
type pbw struct{ b []byte }

func (w *pbw) key(f, wire int)      { w.b = binary.AppendUvarint(w.b, uint64(f<<3|wire)) }
func (w *pbw) uint(f int, v uint64) { w.key(f, wireVarint); w.b = binary.AppendUvarint(w.b, v) }
func (w *pbw) int(f int, v int64)   { w.uint(f, uint64(v)) }
func (w *pbw) sint(f int, v int64)  { w.uint(f, uint64((v<<1)^(v>>63))) }
func (w *pbw) bytes(f int, b []byte) {
	w.key(f, wireBytes)
	w.b = binary.AppendUvarint(w.b, uint64(len(b)))
	w.b = append(w.b, b...)
}
func (w *pbw) msg(f int, fn func(*pbw)) { var m pbw; fn(&m); w.bytes(f, m.b) }
func packSint(vs ...int64) []byte {
	var b []byte
	prev := int64(0)
	for _, v := range vs {
		d := v - prev
		prev = v
		b = binary.AppendUvarint(b, uint64((d<<1)^(d>>63)))
	}
	return b
}
func packUint(vs ...uint64) []byte {
	var b []byte
	for _, v := range vs {
		b = binary.AppendUvarint(b, v)
	}
	return b
}

func writeBlob(out *bytes.Buffer, kind string, raw []byte, compress bool) {
	var blob pbw
	if compress {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		blob.int(2, int64(len(raw)))
		blob.bytes(3, z.Bytes())
	} else {
		blob.bytes(1, raw)
	}
	var head pbw
	head.bytes(1, []byte(kind))
	head.int(3, int64(len(blob.b)))
	_ = binary.Write(out, binary.BigEndian, uint32(len(head.b)))
	out.Write(head.b)
	out.Write(blob.b)
}

func fixture(requiredFeature string) []byte {
	var out bytes.Buffer
	var header pbw
	header.msg(1, func(m *pbw) {
		m.sint(1, 40_000_000_000)
		m.sint(2, 41_000_000_000)
		m.sint(3, 38_000_000_000)
		m.sint(4, 37_000_000_000)
	})
	header.bytes(4, []byte(requiredFeature))
	header.bytes(16, []byte("osmctl-test"))
	header.int(32, 1_760_000_000)
	writeBlob(&out, "OSMHeader", header.b, false)

	// Strings: 0 is reserved empty; 1 amenity, 2 cafe, 3 alice, 4 highway, 5 path, 6 outer, 7 name, 8 Kafe.
	strs := []string{"", "amenity", "cafe", "alice", "highway", "path", "outer", "name", "Kafe"}
	var block pbw
	block.msg(1, func(st *pbw) {
		for _, s := range strs {
			st.bytes(1, []byte(s))
		}
	})
	block.msg(2, func(g *pbw) {
		g.msg(2, func(d *pbw) {
			d.bytes(1, packSint(1, 2, 5))
			d.msg(5, func(i *pbw) {
				i.bytes(1, packUint(3, 1, 1))
				i.bytes(2, packSint(1_700_000_000, 1_700_000_100, 1_700_000_100))
				i.bytes(3, packSint(9, 9, 10))
				i.bytes(4, packSint(42, 42, 7))
				i.bytes(5, packSint(3, 3, 0))
			})
			// Granularity 100 nanodegrees: 37.91 = 379100000 * 100.
			d.bytes(8, packSint(379_100_000, 379_110_000, -1))
			d.bytes(9, packSint(402_300_000, 402_310_000, 0))
			d.bytes(10, packUint(1, 2, 7, 8, 0, 0, 0))
		})
	})
	block.msg(2, func(g *pbw) {
		g.msg(1, func(n *pbw) {
			n.sint(1, 6)
			n.bytes(2, packUint(4))
			n.bytes(3, packUint(5))
			n.sint(8, 100)
			n.sint(9, -200)
		})
	})
	block.msg(2, func(g *pbw) {
		g.msg(3, func(w *pbw) {
			w.int(1, 10)
			w.bytes(2, packUint(4))
			w.bytes(3, packUint(5))
			w.msg(4, func(i *pbw) { i.int(1, 2); i.int(2, 1_700_000_200); i.int(3, 11); i.int(4, 42); i.int(5, 3) })
			w.bytes(8, packSint(1, 2, 1))
		})
		g.msg(4, func(r *pbw) {
			r.int(1, 20)
			r.bytes(8, packUint(6, 0))
			r.bytes(9, packSint(10, 1))
			r.bytes(10, packUint(1, 0))
		})
	})
	writeBlob(&out, "OSMData", block.b, true)
	return out.Bytes()
}

func readAll(t *testing.T, data []byte, nodesOnly bool) (*PBFReader, *Data) {
	t.Helper()
	r := NewPBFReader(bytes.NewReader(data))
	r.NodesOnly = nodesOnly
	d := &Data{}
	if err := r.Each(context.Background(), func(el Element) error { d.Add(el); return nil }); err != nil {
		t.Fatal(err)
	}
	return r, d
}

func TestPBF(t *testing.T) {
	r, d := readAll(t, fixture("DenseNodes"), false)

	if h := r.Header; h.WritingProgram != "osmctl-test" || h.Bounds.MinLon != 40 || h.Bounds.MaxLat != 38 || h.ReplicationTimestamp.Unix() != 1_760_000_000 {
		t.Errorf("header = %+v", h)
	}
	if len(d.Nodes) != 4 || len(d.Ways) != 1 || len(d.Relations) != 1 {
		t.Fatalf("decoded %d/%d/%d", len(d.Nodes), len(d.Ways), len(d.Relations))
	}

	n := d.Nodes[0]
	if n.ID != 1 || n.Lat != 37.91 || n.Lon != 40.23 || n.Tags.Find("amenity") != "cafe" || n.Tags.Find("name") != "Kafe" {
		t.Errorf("dense node 1 = %+v", n)
	}
	if n.Version != 3 || n.User != "alice" || n.UID != 42 || n.Changeset != 9 || !n.Timestamp.Equal(time.Unix(1_700_000_000, 0)) || !n.Visible {
		t.Errorf("dense node 1 meta = %+v", n.Meta)
	}
	if n2 := d.Nodes[1]; n2.ID != 2 || n2.Lat != 37.911 || len(n2.Tags) != 0 || n2.UID != 42 {
		t.Errorf("dense node 2 = %+v", n2)
	}
	if n5 := d.Nodes[2]; n5.ID != 5 || n5.Lat != -1e-7 || n5.Changeset != 10 || n5.UID != 7 {
		t.Errorf("dense node 5 = %+v", n5)
	}
	if n6 := d.Nodes[3]; n6.ID != 6 || n6.Lat != 1e-5 || n6.Lon != -2e-5 || n6.Tags.Find("highway") != "path" {
		t.Errorf("plain node = %+v", n6)
	}

	w := d.Ways[0]
	if w.ID != 10 || len(w.Nodes) != 3 || w.Nodes[0] != 1 || w.Nodes[1] != 2 || w.Nodes[2] != 1 || w.Version != 2 || w.User != "alice" || w.Tags.Find("highway") != "path" {
		t.Errorf("way = %+v", w)
	}
	rel := d.Relations[0]
	if len(rel.Members) != 2 || rel.Members[0] != (Member{TypeWay, 10, "outer"}) || rel.Members[1] != (Member{TypeNode, 1, ""}) {
		t.Errorf("relation = %+v", rel)
	}
}

func TestPBFNodesOnly(t *testing.T) {
	_, d := readAll(t, fixture("DenseNodes"), true)
	if len(d.Nodes) != 4 || len(d.Ways)+len(d.Relations) != 0 {
		t.Errorf("nodes only: %d/%d/%d", len(d.Nodes), len(d.Ways), len(d.Relations))
	}
}

func TestPBFRejectsBadInput(t *testing.T) {
	if err := NewPBFReader(bytes.NewReader(fixture("Unknown-Feature"))).Each(context.Background(), func(Element) error { return nil }); err == nil {
		t.Error("expected an error for an unsupported required feature")
	}
	data := fixture("DenseNodes")
	err := NewPBFReader(bytes.NewReader(data[:len(data)-10])).Each(context.Background(), func(Element) error { return nil })
	if err == nil {
		t.Error("expected an error for a truncated file")
	}
	stop := errors.New("stop")
	err = NewPBFReader(bytes.NewReader(data)).Each(context.Background(), func(Element) error { return stop })
	if !errors.Is(err, stop) {
		t.Errorf("callback error not returned: %v", err)
	}
}

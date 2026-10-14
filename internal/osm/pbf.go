package osm

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"time"
)

// PBFHeader is the file header of an .osm.pbf.
type PBFHeader struct {
	Bounds               *Bounds
	WritingProgram       string
	Source               string
	ReplicationTimestamp time.Time
}

// PBFReader streams the elements of an .osm.pbf file.
// Spec: https://wiki.openstreetmap.org/wiki/PBF_Format
type PBFReader struct {
	r io.Reader
	// NodesOnly skips decoding ways and relations, for a fast pass that only
	// needs coordinates.
	NodesOnly bool
	// Header is set once the file header has been read by Each.
	Header PBFHeader
}

func NewPBFReader(r io.Reader) *PBFReader { return &PBFReader{r: r} }

const (
	maxBlobHeader = 64 * 1024
	maxBlob       = 32 * 1024 * 1024
)

// LocationsOnWays files still carry node refs on ways, so they read fine.
var supportedFeatures = map[string]bool{"OsmSchema-V0.6": true, "DenseNodes": true, "HistoricalInformation": true, "LocationsOnWays": true}

type blob struct {
	kind string
	data []byte
}

func (p *PBFReader) readBlob() (*blob, error) {
	var size [4]byte
	if _, err := io.ReadFull(p.r, size[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n > maxBlobHeader {
		return nil, fmt.Errorf("pbf: blob header of %d bytes exceeds the limit", n)
	}
	head := make([]byte, n)
	if _, err := io.ReadFull(p.r, head); err != nil {
		return nil, unexpectedEOF(err)
	}
	var kind string
	var dataSize int64
	m := pb{head}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			b, err := m.bytes()
			if err != nil {
				return nil, err
			}
			kind = string(b)
		case 3:
			if dataSize, err = m.varint(); err != nil {
				return nil, err
			}
		default:
			if err := m.skip(w); err != nil {
				return nil, err
			}
		}
	}
	if dataSize < 0 || dataSize > maxBlob {
		return nil, fmt.Errorf("pbf: blob of %d bytes exceeds the limit", dataSize)
	}
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(p.r, data); err != nil {
		return nil, unexpectedEOF(err)
	}
	return &blob{kind, data}, nil
}

func unexpectedEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// inflate returns the uncompressed content of a Blob message.
func inflate(data []byte) ([]byte, error) {
	m := pb{data}
	var raw, zdata []byte
	var rawSize int64
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			raw, err = m.bytes()
		case 2:
			rawSize, err = m.varint()
		case 3:
			zdata, err = m.bytes()
		case 4, 5, 6, 7:
			return nil, fmt.Errorf("pbf: compression type %d is not supported (only zlib and raw)", f)
		default:
			err = m.skip(w)
		}
		if err != nil {
			return nil, err
		}
	}
	if raw != nil {
		return raw, nil
	}
	if rawSize < 0 || rawSize > maxBlob {
		return nil, fmt.Errorf("pbf: uncompressed blob of %d bytes exceeds the limit", rawSize)
	}
	zr, err := zlib.NewReader(bytes.NewReader(zdata))
	if err != nil {
		return nil, fmt.Errorf("pbf: %w", err)
	}
	out := make([]byte, 0, rawSize)
	buf := bytes.NewBuffer(out)
	if _, err := io.Copy(buf, io.LimitReader(zr, maxBlob+1)); err != nil {
		return nil, fmt.Errorf("pbf: %w", err)
	}
	if buf.Len() > maxBlob {
		return nil, errors.New("pbf: uncompressed blob exceeds the limit")
	}
	return buf.Bytes(), zr.Close()
}

func (p *PBFReader) readHeader(data []byte) error {
	m := pb{data}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return err
		}
		switch f {
		case 1:
			b, err := m.bytes()
			if err != nil {
				return err
			}
			p.Header.Bounds, err = headerBBox(b)
			if err != nil {
				return err
			}
		case 4:
			b, err := m.bytes()
			if err != nil {
				return err
			}
			if !supportedFeatures[string(b)] {
				return fmt.Errorf("pbf: file requires unsupported feature %q", b)
			}
		case 16, 17:
			b, err := m.bytes()
			if err != nil {
				return err
			}
			if f == 16 {
				p.Header.WritingProgram = string(b)
			} else {
				p.Header.Source = string(b)
			}
		case 32:
			ts, err := m.varint()
			if err != nil {
				return err
			}
			p.Header.ReplicationTimestamp = time.Unix(ts, 0).UTC()
		default:
			if err := m.skip(w); err != nil {
				return err
			}
		}
	}
	return nil
}

func headerBBox(b []byte) (*Bounds, error) {
	m := pb{b}
	var v [5]int64
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		if f >= 1 && f <= 4 {
			if v[f], err = m.sint(); err != nil {
				return nil, err
			}
			continue
		}
		if err := m.skip(w); err != nil {
			return nil, err
		}
	}
	// Fields are left, right, top, bottom in nanodegrees.
	return &Bounds{MinLon: float64(v[1]) / 1e9, MaxLon: float64(v[2]) / 1e9, MaxLat: float64(v[3]) / 1e9, MinLat: float64(v[4]) / 1e9}, nil
}

type decoded struct {
	els []Element
	err error
}

// Each calls fn for every element, in file order. Blocks are decompressed and
// decoded on all cores; results are handed to fn in sequence.
func (p *PBFReader) Each(ctx context.Context, fn func(Element) error) error {
	first, err := p.readBlob()
	if err != nil {
		return fmt.Errorf("pbf: read header: %w", unexpectedEOF(err))
	}
	if first.kind != "OSMHeader" {
		return fmt.Errorf("pbf: expected OSMHeader, found %q", first.kind)
	}
	data, err := inflate(first.data)
	if err != nil {
		return err
	}
	if err := p.readHeader(data); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	workers := runtime.GOMAXPROCS(0)
	jobs := make(chan func(), workers)
	order := make(chan chan decoded, workers*2)
	readErr := make(chan error, 1)

	for range workers {
		go func() {
			for job := range jobs {
				job()
			}
		}()
	}
	go func() {
		defer close(order)
		defer close(jobs)
		for {
			b, err := p.readBlob()
			if errors.Is(err, io.EOF) {
				readErr <- nil
				return
			}
			if err != nil {
				readErr <- err
				return
			}
			if b.kind != "OSMData" {
				continue
			}
			out := make(chan decoded, 1)
			select {
			case order <- out:
			case <-ctx.Done():
				readErr <- ctx.Err()
				return
			}
			jobs <- func() {
				raw, err := inflate(b.data)
				if err != nil {
					out <- decoded{err: err}
					return
				}
				els, err := decodeBlock(raw, p.NodesOnly)
				out <- decoded{els, err}
			}
		}
	}()

	for out := range order {
		res := <-out
		if res.err != nil {
			return res.err
		}
		for _, el := range res.els {
			if err := fn(el); err != nil {
				return err
			}
		}
	}
	return <-readErr
}

type block struct {
	strings         [][]byte
	granularity     int64
	dateGranularity int64
	latOffset       int64
	lonOffset       int64
	nodesOnly       bool
	out             []Element
}

func decodeBlock(data []byte, nodesOnly bool) ([]Element, error) {
	b := &block{granularity: 100, dateGranularity: 1000, nodesOnly: nodesOnly}
	var groups [][]byte
	m := pb{data}
	// Groups come before the coordinate settings on the wire, so collect them
	// first and decode once the whole block header is known.
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			st, err := m.bytes()
			if err != nil {
				return nil, err
			}
			if b.strings, err = stringTable(st); err != nil {
				return nil, err
			}
		case 2:
			g, err := m.bytes()
			if err != nil {
				return nil, err
			}
			groups = append(groups, g)
		case 17:
			b.granularity, err = m.varint()
		case 18:
			b.dateGranularity, err = m.varint()
		case 19:
			b.latOffset, err = m.varint()
		case 20:
			b.lonOffset, err = m.varint()
		default:
			err = m.skip(w)
		}
		if err != nil {
			return nil, err
		}
	}
	for _, g := range groups {
		if err := b.group(g); err != nil {
			return nil, err
		}
	}
	return b.out, nil
}

func stringTable(data []byte) ([][]byte, error) {
	m := pb{data}
	var out [][]byte
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		if f != 1 {
			if err := m.skip(w); err != nil {
				return nil, err
			}
			continue
		}
		s, err := m.bytes()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (b *block) str(i int64) (string, error) {
	if i < 0 || i >= int64(len(b.strings)) {
		return "", fmt.Errorf("pbf: string index %d out of range", i)
	}
	return string(b.strings[i]), nil
}

func (b *block) coord(offset, v int64) float64 {
	return float64(offset+b.granularity*v) / 1e9
}

func (b *block) group(data []byte) error {
	m := pb{data}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return err
		}
		var msg []byte
		if w == wireBytes {
			if msg, err = m.bytes(); err != nil {
				return err
			}
		} else if err := m.skip(w); err != nil {
			return err
		}
		switch {
		case f == 1:
			err = b.node(msg)
		case f == 2:
			err = b.dense(msg)
		case f == 3 && !b.nodesOnly:
			err = b.way(msg)
		case f == 4 && !b.nodesOnly:
			err = b.relation(msg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *block) tags(keys, vals []uint32) (Tags, error) {
	if len(keys) != len(vals) {
		return nil, errors.New("pbf: tag keys and values differ in length")
	}
	if len(keys) == 0 {
		return nil, nil
	}
	ts := make(Tags, len(keys))
	for i := range keys {
		k, err := b.str(int64(keys[i]))
		if err != nil {
			return nil, err
		}
		v, err := b.str(int64(vals[i]))
		if err != nil {
			return nil, err
		}
		ts[i] = Tag{k, v}
	}
	return ts, nil
}

func (b *block) info(data []byte) (Meta, error) {
	meta := Meta{Visible: true}
	m := pb{data}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return meta, err
		}
		var v int64
		switch f {
		case 1:
			v, err = m.varint()
			meta.Version = int(v)
		case 2:
			v, err = m.varint()
			meta.Timestamp = time.UnixMilli(v * b.dateGranularity).UTC()
		case 3:
			meta.Changeset, err = m.varint()
		case 4:
			meta.UID, err = m.varint()
		case 5:
			if v, err = m.varint(); err == nil {
				meta.User, err = b.str(v)
			}
		case 6:
			v, err = m.varint()
			meta.Visible = v != 0
		default:
			err = m.skip(w)
		}
		if err != nil {
			return meta, err
		}
	}
	return meta, nil
}

// common reads the fields nodes, ways and relations share: id, tags and info.
// Anything else is returned by field number for the caller.
func (b *block) common(data []byte) (id int64, tags Tags, meta Meta, rest map[int][]byte, err error) {
	var keys, vals []uint32
	meta = Meta{Visible: true}
	rest = map[int][]byte{}
	m := pb{data}
	for !m.done() {
		f, w, ferr := m.field()
		if ferr != nil {
			return 0, nil, meta, nil, ferr
		}
		switch {
		case f == 1:
			id, err = m.varint()
		case f == 2 || f == 3 || f == 4:
			var raw []byte
			if raw, err = m.bytes(); err != nil {
				break
			}
			switch f {
			case 2:
				keys, err = packed(raw, asUint32)
			case 3:
				vals, err = packed(raw, asUint32)
			case 4:
				meta, err = b.info(raw)
			}
		case w == wireVarint:
			var v uint64
			v, err = m.uvarint()
			rest[f] = binary.AppendUvarint(nil, v)
		case w == wireBytes:
			rest[f], err = m.bytes()
		default:
			err = m.skip(w)
		}
		if err != nil {
			return 0, nil, meta, nil, err
		}
	}
	tags, err = b.tags(keys, vals)
	return id, tags, meta, rest, err
}

func (b *block) node(data []byte) error {
	id, tags, meta, rest, err := b.common(data)
	if err != nil {
		return err
	}
	lat, _ := (&pb{rest[8]}).uvarint()
	lon, _ := (&pb{rest[9]}).uvarint()
	// Plain (non-dense) node ids are signed varints, unlike ways and relations.
	b.out = append(b.out, &Node{ID: zigzag(uint64(id)), Lat: b.coord(b.latOffset, zigzag(lat)), Lon: b.coord(b.lonOffset, zigzag(lon)), Tags: tags, Meta: meta})
	return nil
}

func (b *block) way(data []byte) error {
	id, tags, meta, rest, err := b.common(data)
	if err != nil {
		return err
	}
	refs, err := packed(rest[8], zigzag)
	if err != nil {
		return err
	}
	b.out = append(b.out, &Way{ID: id, Nodes: deltas(refs), Tags: tags, Meta: meta})
	return nil
}

var memberTypes = []Type{TypeNode, TypeWay, TypeRelation}

func (b *block) relation(data []byte) error {
	id, tags, meta, rest, err := b.common(data)
	if err != nil {
		return err
	}
	roles, err := packed(rest[8], asInt)
	if err != nil {
		return err
	}
	ids, err := packed(rest[9], zigzag)
	if err != nil {
		return err
	}
	types, err := packed(rest[10], asInt)
	if err != nil {
		return err
	}
	if len(roles) != len(ids) || len(ids) != len(types) {
		return fmt.Errorf("pbf: relation %d has inconsistent member arrays", id)
	}
	deltas(ids)
	members := make([]Member, len(ids))
	for i := range ids {
		if types[i] < 0 || types[i] > 2 {
			return fmt.Errorf("pbf: relation %d has unknown member type %d", id, types[i])
		}
		role, err := b.str(roles[i])
		if err != nil {
			return err
		}
		members[i] = Member{memberTypes[types[i]], ids[i], role}
	}
	b.out = append(b.out, &Relation{ID: id, Members: members, Tags: tags, Meta: meta})
	return nil
}

func (b *block) dense(data []byte) error {
	var ids, lats, lons []int64
	var keysVals []int64
	var info []byte
	m := pb{data}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return err
		}
		if w != wireBytes {
			if err := m.skip(w); err != nil {
				return err
			}
			continue
		}
		raw, err := m.bytes()
		if err != nil {
			return err
		}
		switch f {
		case 1:
			ids, err = packed(raw, zigzag)
		case 5:
			info = raw
		case 8:
			lats, err = packed(raw, zigzag)
		case 9:
			lons, err = packed(raw, zigzag)
		case 10:
			keysVals, err = packed(raw, asInt)
		}
		if err != nil {
			return err
		}
	}
	if len(lats) != len(ids) || len(lons) != len(ids) {
		return errors.New("pbf: dense nodes have inconsistent arrays")
	}
	deltas(ids)
	deltas(lats)
	deltas(lons)
	metas, err := b.denseInfo(info, len(ids))
	if err != nil {
		return err
	}

	kv := 0
	for i := range ids {
		n := &Node{ID: ids[i], Lat: b.coord(b.latOffset, lats[i]), Lon: b.coord(b.lonOffset, lons[i]), Meta: metas[i]}
		// keys_vals lists key,value pairs per node, each node ending with 0.
		for kv < len(keysVals) && keysVals[kv] != 0 {
			if kv+1 >= len(keysVals) {
				return errors.New("pbf: dense node tags are truncated")
			}
			k, err := b.str(keysVals[kv])
			if err != nil {
				return err
			}
			v, err := b.str(keysVals[kv+1])
			if err != nil {
				return err
			}
			n.Tags = append(n.Tags, Tag{k, v})
			kv += 2
		}
		kv++
		b.out = append(b.out, n)
	}
	return nil
}

func (b *block) denseInfo(data []byte, n int) ([]Meta, error) {
	metas := make([]Meta, n)
	for i := range metas {
		metas[i].Visible = true
	}
	if data == nil {
		return metas, nil
	}
	var versions, times, changesets, uids, users []int64
	var visible []bool
	m := pb{data}
	for !m.done() {
		f, w, err := m.field()
		if err != nil {
			return nil, err
		}
		if w != wireBytes {
			if err := m.skip(w); err != nil {
				return nil, err
			}
			continue
		}
		raw, err := m.bytes()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			versions, err = packed(raw, asInt)
		case 2:
			times, err = packed(raw, zigzag)
		case 3:
			changesets, err = packed(raw, zigzag)
		case 4:
			uids, err = packed(raw, zigzag)
		case 5:
			users, err = packed(raw, zigzag)
		case 6:
			visible, err = packed(raw, asBool)
		}
		if err != nil {
			return nil, err
		}
	}
	deltas(times)
	deltas(changesets)
	deltas(uids)
	deltas(users)
	for i := range metas {
		if i < len(versions) {
			metas[i].Version = int(versions[i])
		}
		if i < len(times) {
			metas[i].Timestamp = time.UnixMilli(times[i] * b.dateGranularity).UTC()
		}
		if i < len(changesets) {
			metas[i].Changeset = changesets[i]
		}
		if i < len(uids) {
			metas[i].UID = uids[i]
		}
		if i < len(users) {
			u, err := b.str(users[i])
			if err != nil {
				return nil, err
			}
			metas[i].User = u
		}
		if i < len(visible) {
			metas[i].Visible = visible[i]
		}
	}
	return metas, nil
}

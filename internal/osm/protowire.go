package osm

import (
	"errors"
	"fmt"
)

// A minimal protobuf wire-format reader: the OSM PBF schema only needs
// varints, zigzag integers and length-delimited fields.

var errTruncated = errors.New("pbf: truncated protobuf message")

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

type pb struct {
	b []byte
}

func (p *pb) done() bool { return len(p.b) == 0 }

func (p *pb) uvarint() (uint64, error) {
	var v uint64
	for i := range 10 {
		if i >= len(p.b) {
			return 0, errTruncated
		}
		c := p.b[i]
		v |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			p.b = p.b[i+1:]
			return v, nil
		}
	}
	return 0, errors.New("pbf: varint overflow")
}

func (p *pb) varint() (int64, error) {
	v, err := p.uvarint()
	return int64(v), err
}

func (p *pb) sint() (int64, error) {
	v, err := p.uvarint()
	return zigzag(v), err
}

func zigzag(v uint64) int64 { return int64(v>>1) ^ -int64(v&1) }

func (p *pb) bytes() ([]byte, error) {
	n, err := p.uvarint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(p.b)) {
		return nil, errTruncated
	}
	b := p.b[:n]
	p.b = p.b[n:]
	return b, nil
}

// field reads the next key. Callers handle the fields they know and skip the rest.
func (p *pb) field() (num int, wire int, err error) {
	k, err := p.uvarint()
	if err != nil {
		return 0, 0, err
	}
	return int(k >> 3), int(k & 7), nil
}

func (p *pb) skip(wire int) error {
	switch wire {
	case wireVarint:
		_, err := p.uvarint()
		return err
	case wireFixed64:
		return p.advance(8)
	case wireBytes:
		_, err := p.bytes()
		return err
	case wireFixed32:
		return p.advance(4)
	}
	return fmt.Errorf("pbf: unsupported wire type %d", wire)
}

func (p *pb) advance(n int) error {
	if n > len(p.b) {
		return errTruncated
	}
	p.b = p.b[n:]
	return nil
}

// packed decodes a packed repeated varint field, applying conv to each value.
func packed[T any](b []byte, conv func(uint64) T) ([]T, error) {
	p := pb{b}
	var out []T
	for !p.done() {
		v, err := p.uvarint()
		if err != nil {
			return nil, err
		}
		out = append(out, conv(v))
	}
	return out, nil
}

func asInt(v uint64) int64     { return int64(v) }
func asUint32(v uint64) uint32 { return uint32(v) }
func asBool(v uint64) bool     { return v != 0 }

// deltas undoes delta coding in place: PBF stores ids, coordinates and
// timestamps as differences from the previous value.
func deltas(vs []int64) []int64 {
	for i := 1; i < len(vs); i++ {
		vs[i] += vs[i-1]
	}
	return vs
}

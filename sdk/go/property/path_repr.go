// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package property

import (
	"encoding/binary"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// The underlying data representation of a [Path] or [Glob].
//
// It is stored in a string so it is immutable & comparable, but its a binary format. Each segment has a header that
// determines byte length with:
//
// 00 - inline index (8 bits) with an index up to 64
// 01 - splat - 8 bits (header)
// 10 - string - 14 bits (header + length) + content (string of specified length, max length 2^14)
// 11 - index - 8 bits (header) + 32 bits (number)
type pathRepr struct{ string }

func (p pathRepr) appendGlobSegment(segment GlobSegment) pathRepr {
	switch segment := segment.(type) {
	case KeySegment:
		return p.appendKey(segment.string)
	case IndexSegment:
		return p.appendIndex(segment.i)
	case splat:
		return p.appendSplat()
	default:
		contract.Failf("unexpected glob segment %T", segment)
		return p
	}
}

func (p pathRepr) appendSplat() pathRepr {
	return pathRepr{p.string + string([]byte{0x40})}
}

func (p pathRepr) appendKey(s string) pathRepr {
	contract.Assertf(len(s) < 1<<14, "string exceeded max length")
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(2<<14|len(s))) //nolint:gosec // checked above
	return pathRepr{p.string + string(hdr[:]) + s}
}

func (p pathRepr) appendIndex(i uint64) pathRepr {
	if i < 64 {
		return pathRepr{p.string + string([]byte{byte(i)})} //nolint:gosec // checked above
	}
	var buf [9]byte
	buf[0] = 0xC0 //nolint:gosec // https://github.com/securego/gosec/issues/1495 (fixed in golangci-lint 2.10.0)
	binary.BigEndian.PutUint64(buf[1:], i)
	return pathRepr{p.string + string(buf[:])}
}

// S should be PathSegment or GlobSegment
func pathReprFromSegments[S any](segments []S) (ret pathRepr) {
	for _, s := range segments {
		switch s := any(s).(type) {
		case KeySegment:
			ret = ret.appendKey(s.string)
		case IndexSegment:
			ret = ret.appendIndex(s.i)
		case splat:
			ret = ret.appendSplat()
		default:
			contract.Failf("unknown segment type %T", s)
		}
	}
	return ret
}

func (p pathRepr) len() (i int) {
	for range p.segments {
		i++
	}
	return i
}

func (p pathRepr) enumerate(yield func(int, GlobSegment) bool) {
	i := 0
	for v := range p.segments {
		if !yield(i, v) {
			return
		}
		i++
	}
}

func (p pathRepr) segments(yield func(GlobSegment) bool) {
	s := p.string
	for len(s) > 0 {
		b := s[0]
		switch b >> 6 {
		case 0:
			// Inline index: lower 6 bits hold the value (0-63).
			s = s[1:]
			if !yield(IndexSegment{uint64(b & 0x3F)}) {
				return
			}
		case 1:
			// Splat: single byte header.
			s = s[1:]
			if !yield(Splat) {
				return
			}
		case 2:
			// String segment: 2-byte header, lower 14 bits are length.
			contract.Assertf(len(s) >= 2, "unexpected end of path representation")
			hdr := binary.BigEndian.Uint16([]byte(s[:2]))
			s = s[2:]
			n := int(hdr & 0x3FFF)
			contract.Assertf(len(s) >= n, "unexpected end of path representation")
			if !yield(KeySegment{s[:n]}) {
				return
			}
			s = s[n:]
		case 3:
			// Large index: 1-byte header + 4-byte uint32.
			contract.Assertf(len(s) >= 5, "unexpected end of path representation")
			idx := binary.BigEndian.Uint64([]byte(s[1:9]))
			s = s[9:]
			if !yield(IndexSegment{idx}) {
				return
			}
		}
	}
}

// segmentByteLen returns the number of bytes occupied by the segment starting at s[offset].
func segmentByteLen(s string, offset int) int {
	b := s[offset]
	switch b >> 6 {
	case 0, 1:
		return 1
	case 2:
		hdr := binary.BigEndian.Uint16([]byte(s[offset : offset+2]))
		return 2 + int(hdr&0x3FFF)
	case 3:
		return 9
	default:
		contract.Failf("unexpected segment header byte: %x", b)
		return 0
	}
}

// split splits the path at the given segment index, returning the segments before and
// after idx as separate pathReprs.
func (p pathRepr) split(idx int) (pathRepr, pathRepr) {
	offset := 0
	for range idx {
		contract.Assertf(offset < len(p.string), "split index %d out of range", idx)
		offset += segmentByteLen(p.string, offset)
	}
	return pathRepr{p.string[:offset]}, pathRepr{p.string[offset:]}
}

func (p pathRepr) last() (pathRepr, GlobSegment) {
	contract.Assertf(len(p.string) > 0, "last called on empty path")
	prefix, suffix := p.split(p.len() - 1)
	var seg GlobSegment
	for s := range suffix.segments {
		seg = s
	}
	return prefix, seg
}

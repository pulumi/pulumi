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
// 00 - not used
// 01 - splat - 8 bits (header)
// 10 - string - 14 bits (header + length) + content (string of specified length, max length 2^14)
// 11 - index - 2 bits (header) + 14 bits (length)
type pathRepr struct{ string }

func (p pathRepr) appendGlobSegment(segment GlobSegment) pathRepr {
	switch segment := segment.(type) {
	case KeySegment:
		return p.appendKey(segment.string)
	case IndexSegment:
		return p.appendIndex(segment.int)
	case splat:
		return p.appendSplat()
	default:
		contract.Failf("unexpected glob segment %T", segment)
		return p
	}
}

func (p pathRepr) appendSplat() pathRepr {
	return pathRepr{p.string + "\x04"}
}

func (p pathRepr) appendKey(s string) pathRepr {
	contract.Assertf(len(s) < 1<<14, "string exceeded max length")
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(2<<14|len(s))) //nolint:gosec // checked above
	return pathRepr{p.string + string(hdr[:]) + s}
}

func (p pathRepr) appendIndex(i int) pathRepr {
	contract.Assertf(i < 1<<14, "index exceeded max length")
	contract.Assertf(i >= 0, "index must be non-negative")
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(3<<14|i)) //nolint:gosec // checked above
	return pathRepr{p.string + string(hdr[:])}
}

// S should be PathSegment or GlobSegment
func pathReprFromSegments[S any](segments []S) (ret pathRepr) {
	for _, s := range segments {
		switch s := any(s).(type) {
		case KeySegment:
			ret = ret.appendKey(s.string)
		case IndexSegment:
			ret = ret.appendIndex(s.int)
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
	i := 0
	for len(s) > 0 {
		b := s[0]
		switch {
		case b < 0x80:
			// Splat: single byte header.
			contract.Assertf(b == 0x04, "unexpected splat byte: %x", b)
			s = s[1:]
			if !yield(Splat) {
				return
			}
		default:
			// 2-byte header.
			contract.Assertf(len(s) >= 2, "unexpected end of path representation")
			hdr := binary.BigEndian.Uint16([]byte(s[:2]))
			s = s[2:]
			switch hdr >> 14 {
			case 2:
				// String segment.
				n := int(hdr & 0x3FFF)
				contract.Assertf(len(s) >= n, "unexpected end of path representation")
				if !yield(KeySegment{s[:n]}) {
					return
				}
				s = s[n:]
			case 3:
				// Index segment.
				if !yield(IndexSegment{int(hdr & 0x3FFF)}) {
					return
				}
			default:
				contract.Failf("unexpected path segment header: %x", hdr)
			}
		}
		i++
	}
}

// segmentByteLen returns the number of bytes occupied by the segment starting at s[offset].
func segmentByteLen(s string, offset int) int {
	b := s[offset]
	if b < 0x80 {
		return 1
	}
	n := 2
	hdr := binary.BigEndian.Uint16([]byte(s[offset : offset+2]))
	if hdr>>14 == 2 {
		n += int(hdr & 0x3FFF)
	}
	return n
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

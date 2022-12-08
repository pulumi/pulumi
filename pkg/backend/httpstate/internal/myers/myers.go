// Copyright 2016-2022, Pulumi Corporation.
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

// Provides a streaming MyersComputeEdits that can be used in place of myers.ComputeEdits from
// github.com/hexops/gotextdiff/myers.
package myers

import (
	"io"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

// Computes the edits necessary to convert before text to after text.
//
// The advantage of this version over the original from gotextdiff is that it works in a streaming fashion over
// io.Reader and will use less memory on large documents.
//
// It also no longer requires the input to have newlines to generate efficient diffs. Instead of injecting newlines, set
// bufsize to a value that is smaller than the document size.
//
// If the bufsize is too small it can make the computed edits less efficient.
func MyersComputeEdits(before, after io.Reader, bufsize int) ([]gotextdiff.TextEdit, error) {
	uri := span.URIFromURI("")
	bR, aR := newReader(before, bufsize), newReader(after, bufsize)
	edits := []gotextdiff.TextEdit{}
	currentOffset := 0
	for {
		bslice, err := bR.read()
		if err != nil {
			return nil, err
		}
		aslice, err := aR.read()
		if err != nil {
			return nil, err
		}
		if aR.eof && bR.eof && len(aslice) == 0 && len(bslice) == 0 {
			return edits, nil
		}
		sliceEdits := myers.ComputeEdits(uri, string(bslice), string(aslice))
		shiftedEdits, err := shiftEdits(currentOffset, bslice, sliceEdits)
		if err != nil {
			return nil, err
		}

		// In case of large inserts, pause reading from before reader, so remainder of the before document has a
		// chance to continue diffing with the rest of the after document. See TestLargeInserts.
		if replace, ok := recognizeFullReplace(shiftedEdits, currentOffset, bufsize); ok {
			bR.unread(bslice)
			here := span.NewPoint(0, 0, currentOffset)
			hereSpan := span.New(uri, here, here)
			edits = append(edits, gotextdiff.TextEdit{
				Span:    hereSpan,
				NewText: replace.insert.NewText,
			})
		} else {
			edits = append(edits, shiftedEdits...)
			currentOffset += len(bslice)
		}
	}
}

type replace struct {
	delete gotextdiff.TextEdit
	insert gotextdiff.TextEdit
}

func recognizeFullReplace(edits []gotextdiff.TextEdit, offset int, bufsize int) (replace, bool) {
	ok := len(edits) == 2 &&
		edits[0].NewText == "" &&
		edits[0].Span.Start().Offset() == offset &&
		edits[0].Span.End().Offset() == offset+bufsize &&
		edits[1].Span.Start().Offset() == offset+bufsize &&
		edits[1].Span.End().Offset() == offset+bufsize &&
		len(edits[1].NewText) == bufsize
	if !ok {
		return replace{}, false
	}
	return replace{delete: edits[0], insert: edits[1]}, true
}

type reader struct {
	r      io.Reader
	buffer []byte
	eof    bool
	cache  []byte
}

// Makes the next call to read skip actually reading and return the same slice as before.
func (r *reader) unread(slice []byte) {
	r.cache = slice
}

func (r *reader) read() ([]byte, error) {
	if r.cache != nil {
		data := r.cache
		r.cache = nil
		return data, nil
	}
	if r.eof {
		return nil, nil
	}
	n, err := r.r.Read(r.buffer)
	if err == io.EOF {
		r.eof = true
	} else if err != nil {
		return nil, err
	}
	return r.buffer[0:n], nil
}

func newReader(r io.Reader, bufsize int) *reader {
	return &reader{buffer: make([]byte, bufsize), r: r}
}

func shiftPoint(offset int, orig span.Point) span.Point {
	return span.NewPoint(0, 0, orig.Offset()+offset)
}

func shiftSpan(offset int, orig span.Span) span.Span {
	return span.New(orig.URI(), shiftPoint(offset, orig.Start()), shiftPoint(offset, orig.End()))
}

func shiftEdits(offset int, before []byte, rawEdits []gotextdiff.TextEdit) ([]gotextdiff.TextEdit, error) {
	c := span.NewContentConverter("", before)
	edits := []gotextdiff.TextEdit{}
	for _, sed := range rawEdits {
		s, err := sed.Span.WithOffset(c)
		if err != nil {
			return nil, err
		}
		sed2 := gotextdiff.TextEdit{
			Span:    shiftSpan(offset, s),
			NewText: sed.NewText,
		}
		edits = append(edits, sed2)
	}
	return edits, nil
}

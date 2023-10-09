// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"io"

	"github.com/hashicorp/hcl/v2"
)

// diagnosticWriter provides an implementation of hcl.DiagnosticWriter that performs automatic fixup of hcl.Pos values
// that are missing byte offsets prior to writing out diagnostics. Doing this here allows the rest of the world to be a
// bit lazy and only provider line and column information, with the caveat that tabs must be treated as representing
// 4 spaces in order for the fixup to work.
type diagnosticWriter struct {
	w     hcl.DiagnosticWriter
	files map[string]*hcl.File // map from filenames to source bytes
	lines map[string][]int     // map from filenames to line offsets
}

func newDiagnosticWriter(w io.Writer, files map[string]*hcl.File, width uint, color bool) hcl.DiagnosticWriter {
	return &diagnosticWriter{
		w:     hcl.NewDiagnosticTextWriter(w, files, width, color),
		files: files,
	}
}

// getIndex retrieves the line -> byte offset index for the given file. If the index does not yet exist and if there is
// source available for the file, the index is created.
func (w *diagnosticWriter) getIndex(filename string) ([]int, bool) {
	if w.lines == nil {
		w.lines = map[string][]int{}
	}

	if lines, ok := w.lines[filename]; ok {
		return lines, true
	}

	f, ok := w.files[filename]
	if !ok {
		return nil, false
	}

	lineStart := 0
	var lines []int
	for offset, b := range f.Bytes {
		if b == '\n' {
			lines, lineStart = append(lines, lineStart), offset+1
		}
	}

	w.lines[filename] = lines
	return lines, true
}

// getOffset returns the byte offset for the given position.
func (w *diagnosticWriter) getOffset(filename string, line, column int) int {
	lines, ok := w.getIndex(filename)
	if !ok {
		return 0
	}
	if line < 1 || line > len(lines) {
		return 0
	}

	// Starting at column 1 and the offset of the given line, increment the column and offset we are at or past the
	// desired column, then return the offset. Tabs count as four columns.
	col, offset := 1, lines[line-1]
	bytes := w.files[filename].Bytes[offset:]
	for _, b := range bytes {
		if col >= column || b == '\n' {
			break
		} else if b == '\t' {
			col += 4
		} else {
			col++
		}
		offset++
	}
	return offset
}

func (w *diagnosticWriter) fixupPosOffset(filename string, pos *hcl.Pos) {
	if pos.Byte == 0 && pos.Line != 0 && pos.Column != 0 {
		pos.Byte = w.getOffset(filename, pos.Line, pos.Column)
	}
}

func (w *diagnosticWriter) fixupRangeOffsets(rng *hcl.Range) {
	if rng != nil {
		w.fixupPosOffset(rng.Filename, &rng.Start)
		w.fixupPosOffset(rng.Filename, &rng.End)
		if rng.End.Byte == 0 {
			rng.End = rng.Start
		}
	}
}

func (w *diagnosticWriter) fixupOffsets(d *hcl.Diagnostic) {
	w.fixupRangeOffsets(d.Subject)
	w.fixupRangeOffsets(d.Context)
}

func (w *diagnosticWriter) WriteDiagnostic(d *hcl.Diagnostic) error {
	w.fixupOffsets(d)
	return w.w.WriteDiagnostic(d)
}

func (w *diagnosticWriter) WriteDiagnostics(d hcl.Diagnostics) error {
	for _, d := range d {
		w.fixupOffsets(d)
	}
	return w.w.WriteDiagnostics(d)
}

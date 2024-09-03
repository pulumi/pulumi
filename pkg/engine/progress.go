// Copyright 2024, Pulumi Corporation.
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

package engine

import (
	"io"
	"time"
)

// Creates a new ReadCloser that reports ProgressEvents as bytes are read and
// when it is closed. A Done ProgressEvent will only be reported once, on the
// first call to Close(). Subsequent calls to Close() will be forwarded to the
// underlying ReadCloser, but will not yield duplicate ProgressEvents.
func NewProgressReportingCloser(
	events eventEmitter,
	typ ProgressType,
	id string,
	message string,
	size int64,
	reportingInterval time.Duration,
	closer io.ReadCloser,
) io.ReadCloser {
	if size == -1 {
		return closer
	}

	return &progressReportingCloser{
		events:            events,
		typ:               typ,
		id:                id,
		message:           message,
		received:          0,
		total:             size,
		lastReported:      time.Now(),
		reportingInterval: reportingInterval,
		closed:            false,
		closer:            closer,
	}
}

// A ReadCloser implementation that reports ProgressEvents to an
// underlying eventEmitter as bytes are read and when it is closed.
type progressReportingCloser struct {
	// The eventEmitter to report progress events to.
	events eventEmitter
	// The type of progress being reported.
	typ ProgressType
	// A unique ID for the download being reported.
	id string
	// A message to include in progress events.
	message string
	// The number of bytes received so far.
	received int64
	// The total number of bytes expected.
	total int64
	// The last time a progress event was reported.
	lastReported time.Time
	// The interval at which progress events should be reported.
	reportingInterval time.Duration
	// True if the underlying ReadCloser has been closed.
	closed bool
	// The underlying ReadCloser to read from.
	closer io.ReadCloser
}

func (d *progressReportingCloser) Read(p []byte) (n int, err error) {
	n, err = d.closer.Read(p)
	if n != 0 {
		d.received += int64(n)

		now := time.Now()
		interval := now.Sub(d.lastReported)

		if interval > d.reportingInterval {
			d.lastReported = now
			d.events.progressEvent(d.typ, d.id, d.message, d.received, d.total, false)
		}
	}

	return
}

func (d *progressReportingCloser) Close() error {
	// We'll always forward the Close() call to the underlying ReadCloser, but
	// we'll only report a Done event once.
	err := d.closer.Close()

	if !d.closed {
		d.events.progressEvent(d.typ, d.id, d.message, d.received, d.total, true)
	}

	d.closed = true
	return err
}

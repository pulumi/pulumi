// Copyright 2016-2018, Pulumi Corporation.
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

func DownloadProgressReporter(
	events eventEmitter, closer io.ReadCloser, size int64,
	downloadType DownloadType, id string, message string,
) io.ReadCloser {
	// If we don't know the size, then we aren't going to report progress
	if size == -1 {
		return closer
	}

	return &downloadProgressCloser{
		events:       events,
		closer:       closer,
		total:        size,
		received:     0,
		downloadType: downloadType,
		id:           id,
		msg:          message,
		lastReported: time.Now(),
	}
}

type downloadProgressCloser struct {
	events       eventEmitter
	closer       io.ReadCloser
	total        int64
	received     int64
	downloadType DownloadType
	id           string
	msg          string
	lastReported time.Time
}

func (d *downloadProgressCloser) Close() error {
	d.events.downloadProgressEvent(d.downloadType, d.id, d.msg, d.received, d.total, true)
	return d.closer.Close()
}

func (d *downloadProgressCloser) Read(p []byte) (n int, err error) {
	read, err := d.closer.Read(p)
	if read != 0 {
		d.received += int64(read)
		now := time.Now()
		interval := now.Sub(d.lastReported)
		if interval.Milliseconds() >= 100 {
			d.lastReported = now
			d.events.downloadProgressEvent(d.downloadType, d.id, d.msg, d.received, d.total, false)
		}
	}
	return read, err
}

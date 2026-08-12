// Copyright 2025, Pulumi Corporation.
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

package diag

import (
	"sync"
)

// MockSink is a thread safe mock implementation of the Sink interface that just records all the messages.
type MockSink struct {
	lock     sync.Mutex
	Messages map[Severity][]MockMessage
}

type MockMessage struct {
	Diag *Diag
	Args []any
}

func (d *MockSink) Logf(sev Severity, dia *Diag, args ...any) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.Messages == nil {
		d.Messages = make(map[Severity][]MockMessage)
	}

	d.Messages[sev] = append(d.Messages[sev], MockMessage{
		Diag: dia,
		Args: args,
	})
}

func (d *MockSink) Debugf(dia *Diag, args ...any) {
	d.Logf(Debug, dia, args...)
}

func (d *MockSink) Infof(dia *Diag, args ...any) {
	d.Logf(Info, dia, args...)
}

func (d *MockSink) Infoerrf(dia *Diag, args ...any) {
	d.Logf(Infoerr, dia, args...)
}

func (d *MockSink) Errorf(dia *Diag, args ...any) {
	d.Logf(Error, dia, args...)
}

func (d *MockSink) Warningf(dia *Diag, args ...any) {
	d.Logf(Warning, dia, args...)
}

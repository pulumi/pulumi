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

package tests

import (
	"fmt"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

// L holds the state for the current language test.
//
// It provides an interface similar to testing.T,
// allowing its use with testing libraries like Testify.
type L struct {
	mu sync.RWMutex // guards the fields below

	// Whether this test has already failed.
	failed bool

	// Messages logged to l.Errorf or l.Logf.
	logs []string

	// Functions marked helpers with L.Helper().

	// These names are from the runtime.Frame.Function field.
	// They're fully qualified with package names.
	helpers mapset.Set[string]
}

// Helper marks the calling function as a test helper function.
// When printing file and line information, that function will be skipped.
func (l *L) Helper() {
	pc, _, _, ok := runtime.Caller(1) // skip this function
	if !ok {
		return // unlikely but not worth panicking over
	}

	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.Function == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.helpers == nil {
		l.helpers = mapset.NewSet[string]()
	}
	l.helpers.Add(frame.Function)
}

// FailNow marks this test as having failed and halts execution.
func (l *L) FailNow() {
	l.Fail()
	runtime.Goexit()
}

// Fail marks this test as failed but keeps executing.
func (l *L) Fail() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failed = true
}

// Failed returns whether this test has failed.
func (l *L) Failed() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.failed
}

// Errorf records the given error message and marks this test as failed.
func (l *L) Errorf(format string, args ...interface{}) {
	l.log(1, fmt.Sprintf(format, args...))
	l.Fail()
}

// Logf records the given message in the L's logs.
func (l *L) Logf(format string, args ...interface{}) {
	l.log(1, fmt.Sprintf(format, args...))
}

// log records the given message in the L's logs.
//
// Skip specifies the number of stack frames to skip
// when recording the caller's location.
// 0 refers to the immediate caller of log.
//
// Typically, when used from an exported method on L,
// most callers will want to pass skip=1 to skip themselves
// and record the location of their caller.
func (l *L) log(skip int, msg string) {
	file, line := "???", 1
	if frame, ok := l.callerFrame(skip + 1); ok {
		file, line = frame.File, frame.Line
	}

	msg = fmt.Sprintf("%s:%d: %s", filepath.Base(file), line, msg)
	l.mu.Lock()
	l.logs = append(l.logs, msg)
	l.mu.Unlock()
}

// Maximal stack depth to search for the caller's frame.
const _maxStackDepth = 50

// callerFrame searches the call stack for the first frame
// that isn't a helper function.
//
// skip specifies the initial number of frames to skip
// with 0 referring to the immediate caller of callerFrame.
func (l *L) callerFrame(skip int) (frame runtime.Frame, ok bool) {
	var pc [_maxStackDepth]uintptr
	n := runtime.Callers(skip+2, pc[:]) // skip runtime.Callers and callerFrame
	if n == 0 {
		return frame, false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	frames := runtime.CallersFrames(pc[:n])
	for {
		frame, more := frames.Next()
		if !l.helpers.Contains(frame.Function) {
			// Not a helper. Use this frame.
			return frame, true
		}
		if !more {
			break
		}
	}
	return frame, false // no non-helper frames found
}

// WithL runs the given function with a new L,
// blocking until the function returns.
//
// It returns the information recorded by the L.
func WithL(f func(*L)) LResult {
	// To be able to implement FailNow in the L,
	// we need to run it in a separate goroutine
	// so that we can call runtime.Goexit.
	done := make(chan struct{})
	var l L
	go func() {
		defer func() {
			if r := recover(); r != nil {
				l.failed = true
				l.logs = append(l.logs,
					fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack()))
			}

			close(done)
		}()

		f(&l)
	}()
	<-done

	return LResult{
		Failed:   l.failed,
		Messages: l.logs,
	}
}

// LResult is the result of running a language test.
type LResult struct {
	// Failed is true if the test failed.
	Failed bool

	// Messages contains the messages logged by the test.
	//
	// This doesn't necessarily mean that the test failed.
	// For example, a test may log debugging information
	// that is only useful when the test fails.
	Messages []string
}

// TestingT is a subset of the testing.T interface.
// [L] implements this interface.
type TestingT interface {
	Helper()
	FailNow()
	Fail()
	Failed() bool
	Errorf(string, ...interface{})
	Logf(string, ...interface{})
}

var (
	_ TestingT         = (*L)(nil)
	_ require.TestingT = (TestingT)(nil) // ensure testify compatibility
)

type LanguageTest struct {
	// TODO: This should be a function so we don't have to load all providers in memory all the time.
	Providers []plugin.Provider

	// stackReferences specifies other stack data that this test depends on.
	StackReferences map[string]resource.PropertyMap

	// runs is a list of test runs to execute.
	Runs []TestRun
}

type TestRun struct {
	Config config.Map
	// This can be used to set a main value for the test.
	Main string
	// TODO: This should just return "string", if == "" then ok, else fail
	Assert func(*L, string, error, *deploy.Snapshot, display.ResourceChanges)
	// updateOptions can be used to set the update options for the engine.
	UpdateOptions engine.UpdateOptions
}

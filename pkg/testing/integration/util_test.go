package integration

import (
	"fmt"
	"testing"
	"time"
)

type fakeTestingT struct {
	logs   []string
	errors []string
}

func (t *fakeTestingT) Logf(format string, args ...interface{}) {
	t.logs = append(t.logs, fmt.Sprintf(format, args...))
}

func (t *fakeTestingT) Errorf(format string, args ...interface{}) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func TestAssertHTTPResultWithRetryNoPanic(t *testing.T) {
	t.Parallel()
	tt := &fakeTestingT{}
	// this just checks it doesn't panic
	AssertHTTPResultWithRetry(tt, "https://localhost:76547/invalid-port-number", nil, time.Second, func(string) bool {
		return true
	})
	// now check there was an error logged for the attempt failing
	if len(tt.errors) != 1 {
		t.Errorf("expected error to have been reported, got %#v", tt.errors)
	}
}

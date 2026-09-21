// Copyright 2026, Pulumi Corporation.  All rights reserved.

package auto

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorDiagnosticsDedupesErrorLevelOnly(t *testing.T) {
	t.Parallel()
	events := []engine.Event{
		engine.NewEvent(engine.DiagEventPayload{Severity: diag.Info, Message: "starting"}),
		engine.NewEvent(engine.DiagEventPayload{Severity: diag.Error, Prefix: "error: ", Message: "Missing required configuration variable 'moderna-services:imageDigest'\n"}),
		engine.NewEvent(engine.DiagEventPayload{Severity: diag.Error, Prefix: "error: ", Message: "Missing required configuration variable 'moderna-services:imageDigest'"}),
		engine.NewEvent(engine.DiagEventPayload{Severity: diag.Error, Message: "  "}),
	}
	got := errorDiagnostics(events)
	require.Equal(t, []string{"error: Missing required configuration variable 'moderna-services:imageDigest'"}, got)
}

func TestMemberErrorCarriesStackAndDiagnostics(t *testing.T) {
	t.Parallel()
	inner := errors.New("BAIL: run bailed")
	err := &MemberError{Stack: "dac-test/moderna-services/prod", Diagnostics: []string{"error: boom"}, err: inner}
	assert.Equal(t, "stack dac-test/moderna-services/prod: BAIL: run bailed\n  error: boom", err.Error())
	assert.True(t, errors.Is(err, inner))
	var target *MemberError
	assert.True(t, errors.As(error(err), &target))
}

func TestErrorDiagnosticsEdgeCases(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("\u00e9", 1500) // 3000 bytes of two-byte runes
	cases := []struct {
		name   string
		events []engine.Event
		check  func(t *testing.T, got []string)
	}{
		{
			name:   "truncates at a rune boundary",
			events: []engine.Event{engine.NewEvent(engine.DiagEventPayload{Severity: diag.Error, Message: long})},
			check: func(t *testing.T, got []string) {
				require.Len(t, got, 1)
				assert.True(t, utf8.ValidString(got[0]))
				assert.True(t, strings.HasSuffix(got[0], "..."))
				assert.LessOrEqual(t, len(got[0]), 2003)
			},
		},
		{
			name: "strips color markup",
			events: []engine.Event{engine.NewEvent(engine.DiagEventPayload{
				Severity: diag.Error, Message: colors.SpecError + "boom" + colors.Reset})},
			check: func(t *testing.T, got []string) { require.Equal(t, []string{"boom"}, got) },
		},
		{
			name:   "ignores a diag event with an unexpected payload",
			events: []engine.Event{{Type: engine.DiagEvent}},
			check:  func(t *testing.T, got []string) { require.Empty(t, got) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, errorDiagnostics(tc.events))
		})
	}
}

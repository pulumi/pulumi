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

package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgeDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"72h", 72 * time.Hour},
		{"1h30m", 90 * time.Minute},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			got, err := parseAgeDuration(c.in)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestParseAgeDuration_Errors(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "yesterday", "-30d", "1.5d", "1d3h"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			_, err := parseAgeDuration(in)
			assert.Error(t, err)
		})
	}
}

func TestParseAgeDuration_MonthYearGuidance(t *testing.T) {
	t.Parallel()

	cases := []string{"1mo", "12mo", "1y", "1Y", "1year", "2years", "1month", "3months"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			_, err := parseAgeDuration(in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "months and years are not supported")
		})
	}
}

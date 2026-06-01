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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var monthYearShortcut = regexp.MustCompile(`(?i)^\d+(mo|months?|y|years?)$`)

// parseAgeDuration accepts Go time.ParseDuration syntax plus d and w suffixes
// for whole-number days and weeks.
func parseAgeDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}
	if monthYearShortcut.MatchString(s) {
		return 0, fmt.Errorf(
			"invalid duration %q: months and years are not supported; use days (e.g. 30d) or weeks (e.g. 4w)",
			s)
	}

	switch s[len(s)-1] {
	case 'd', 'w':
		nRaw := s[:len(s)-1]
		n, err := strconv.Atoi(nRaw)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration %q: expected a non-negative integer before %q",
				s, s[len(s)-1:])
		}

		mult := 24 * time.Hour
		if s[len(s)-1] == 'w' {
			mult = 7 * 24 * time.Hour
		}
		return time.Duration(n) * mult, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid duration %q: must be non-negative", s)
	}
	return d, nil
}

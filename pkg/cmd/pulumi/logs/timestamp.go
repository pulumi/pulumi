// Portions of this file are derived from github.com/moby/moby
// (api/types/time/timestamp.go), Copyright 2013-2018 Docker, Inc.,
// licensed under the Apache License, Version 2.0.
//
// Modifications Copyright 2026, Pulumi Corporation.
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

// getTimestamp and parseTimestamps are vendored from
// github.com/moby/moby/api/types/time so this package does not pull in the
// legacy moby/moby module, which collides with the nested moby/moby/client
// module that testcontainers depends on.

package logs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	rFC3339Local     = "2006-01-02T15:04:05"
	rFC3339NanoLocal = "2006-01-02T15:04:05.999999999"
	dateWithZone     = "2006-01-02Z07:00"
	dateLocal        = "2006-01-02"
)

// getTimestamp tries to parse the given string as a Go duration, then as an
// RFC3339 time, and finally as a Unix timestamp. On success it returns a
// Unix timestamp as a string; otherwise it returns the input unchanged. For
// a duration, the returned timestamp is reference minus that duration.
func getTimestamp(value string, reference time.Time) (string, error) {
	if d, err := time.ParseDuration(value); value != "0" && err == nil {
		return strconv.FormatInt(reference.Add(-d).Unix(), 10), nil
	}

	var format string
	parseInLocation := !strings.ContainsAny(value, "zZ+") && strings.Count(value, "-") != 3

	if strings.Contains(value, ".") {
		if parseInLocation {
			format = rFC3339NanoLocal
		} else {
			format = time.RFC3339Nano
		}
	} else if strings.Contains(value, "T") {
		tcolons := strings.Count(value, ":")
		if !parseInLocation && !strings.ContainsAny(value, "zZ") && tcolons > 0 {
			tcolons--
		}
		if parseInLocation {
			switch tcolons {
			case 0:
				format = "2006-01-02T15"
			case 1:
				format = "2006-01-02T15:04"
			default:
				format = rFC3339Local
			}
		} else {
			switch tcolons {
			case 0:
				format = "2006-01-02T15Z07:00"
			case 1:
				format = "2006-01-02T15:04Z07:00"
			default:
				format = time.RFC3339
			}
		}
	} else if parseInLocation {
		format = dateLocal
	} else {
		format = dateWithZone
	}

	var t time.Time
	var err error
	if parseInLocation {
		t, err = time.ParseInLocation(format, value, time.FixedZone(reference.Zone()))
	} else {
		t, err = time.Parse(format, value)
	}

	if err != nil {
		if strings.Contains(value, "-") {
			return "", err
		}
		if _, _, err := parseTimestamp(value); err != nil {
			return "", fmt.Errorf("failed to parse value as time or duration: %q", value)
		}
		return value, nil
	}

	return fmt.Sprintf("%d.%09d", t.Unix(), int64(t.Nanosecond())), nil
}

// parseTimestamps returns seconds and nanoseconds from a timestamp formatted
// as ("%d.%09d", time.Unix(), int64(time.Nanosecond())). An empty value
// returns (defaultSeconds, 0, nil).
func parseTimestamps(value string, defaultSeconds int64) (int64, int64, error) {
	if value == "" {
		return defaultSeconds, 0, nil
	}
	return parseTimestamp(value)
}

func parseTimestamp(value string) (int64, int64, error) {
	s, n, ok := strings.Cut(value, ".")
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return sec, 0, err
	}
	if !ok {
		return sec, 0, nil
	}
	nsec, err := strconv.ParseInt(n, 10, 64)
	if err != nil {
		return sec, nsec, err
	}
	nsec = int64(float64(nsec) * math.Pow(float64(10), float64(9-len(n))))
	return sec, nsec, nil
}

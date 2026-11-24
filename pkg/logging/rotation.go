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

package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// timestampRe matches the timestamp embedded in log filenames (e.g. "stack-20260401T030000.log").
var timestampRe = regexp.MustCompile(`(\d{8}T\d{6})`)

const (
	defaultMaxAgeDays = 7
	defaultMaxTotalMB = 500
)

// RotateLogs deletes old log files from the given directory.
// Logs are deleted if they are older than maxAge, or if the total
// size exceeds maxTotal (oldest first). Defaults are 7 days and 500 MB,
// overridable via PULUMI_LOG_ROTATION_MAX_AGE_DAYS and
// PULUMI_LOG_ROTATION_MAX_TOTAL_MB.
func RotateLogs(logsDir string) {
	rotateLogs(logsDir, time.Now())
}

func rotateLogs(logsDir string, now time.Time) {
	maxAgeDays := defaultMaxAgeDays
	if v := env.LogRotationMaxAgeDays.Value(); v > 0 {
		maxAgeDays = v
	}
	maxTotalBytes := int64(defaultMaxTotalMB) * 1024 * 1024
	if v := env.LogRotationMaxTotalMB.Value(); v > 0 {
		maxTotalBytes = int64(v) * 1024 * 1024
	}

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		logging.V(5).Infof("log rotation: could not read %s: %v", logsDir, err)
		return
	}

	type logFile struct {
		path      string
		size      int64
		timestamp time.Time
	}

	var files []logFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		m := timestampRe.FindString(e.Name())
		if m == "" {
			continue
		}
		ts, err := time.Parse("20060102T150405", m)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, logFile{
			path:      filepath.Join(logsDir, e.Name()),
			size:      info.Size(),
			timestamp: ts,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].timestamp.Before(files[j].timestamp)
	})

	cutoff := now.Add(-time.Duration(maxAgeDays) * 24 * time.Hour)

	var remaining []logFile
	for _, f := range files {
		if f.timestamp.Before(cutoff) {
			logging.V(7).Infof("log rotation: removing expired %s", f.path)
			if err := os.Remove(f.path); err != nil {
				logging.V(5).Infof("log rotation: could not remove %s: %v", f.path, err)
				remaining = append(remaining, f)
			}
		} else {
			remaining = append(remaining, f)
		}
	}

	// Second pass: if total size exceeds limit, delete oldest until under.
	var totalSize int64
	for _, f := range remaining {
		totalSize += f.size
	}

	for i := 0; i < len(remaining) && totalSize > maxTotalBytes; i++ {
		f := remaining[i]
		logging.V(7).Infof("log rotation: removing %s (total %d > %d)", f.path, totalSize, maxTotalBytes)
		if err := os.Remove(f.path); err != nil {
			logging.V(5).Infof("log rotation: could not remove %s: %v", f.path, err)
		} else {
			totalSize -= f.size
		}
	}
}

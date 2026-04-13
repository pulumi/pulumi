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

//go:build darwin

package nosleep

import (
	"log/slog"
	"os"
	"os/exec"
	"strconv"
)

func keepRunning() DoneFunc {
	// Run caffeinate to keep the system awake.
	//nolint:gosec
	cmd := exec.Command("caffeinate", "-i", "-w", strconv.Itoa(os.Getpid()))
	// we intentionally ignore the error here.  If we can't keep the system awake we still want to continue.
	err := cmd.Start()
	if err != nil {
		slog.Info("Failed to get wake lock", "err", err)
		return func() {}
	}
	slog.Info("Got wake lock (caffeinate)", "pid", cmd.Process.Pid)
	return func() {
		_ = cmd.Process.Kill()
		slog.Info("Released wake lock (caffeinate)", "pid", cmd.Process.Pid)
	}
}

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

//go:build windows
// +build windows

package nosleep

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"golang.org/x/sys/windows"
)

const (
	EsSystemRequired = 0x00000001
	EsContinuous     = 0x80000000
)

func keepRunning() DoneFunc {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	setThreadExecStateProc := kernel32.NewProc("SetThreadExecutionState")
	setThreadExecStateProc.Call(EsSystemRequired | EsContinuous)
	logging.V(5).Infof("Got wake lock (SetThreadExecutionState)")
	return func() {
		// At the end of the long running process, we can allow the system to sleep again.
		setThreadExecStateProc.Call(EsContinuous)
		logging.V(5).Infof("Released wake lock (SetThreadExecutionState)")
	}
}

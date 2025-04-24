// Copyright 2020-2024, Pulumi Corporation.
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

package debug

import "fmt"

type LoggingOptions struct {
	// LogLevel - choose verbosity level of at least 1 (least verbose).
	// If not specified, reverts to default log level.
	// Note - These logs may include sensitive information that is provided from your
	// execution environment to your cloud provider (and which Pulumi may not even
	// itself be aware of).
	LogLevel *uint
	// LogToStdErr specifies that all logs should be sent directly to stderr - making it
	// more accessible and avoiding OS level buffering.
	LogToStdErr bool
	// FlowToPlugins reflects the logging settings to plugins as well.
	FlowToPlugins bool
	// Emit tracing to the specified endpoint. Use the file: scheme to write tracing data to a local file
	Tracing string
	// Print detailed debugging output during resource operations
	Debug bool
}

func AddArgs(debugLogOpts *LoggingOptions, sharedArgs []string) []string {
	if debugLogOpts.LogToStdErr {
		sharedArgs = append(sharedArgs, "--logtostderr")
	}
	if debugLogOpts.LogLevel != nil {
		if *debugLogOpts.LogLevel == 0 {
			*debugLogOpts.LogLevel = 1
		}
		sharedArgs = append(sharedArgs, fmt.Sprintf("-v=%d", *debugLogOpts.LogLevel))
	}
	if debugLogOpts.FlowToPlugins {
		sharedArgs = append(sharedArgs, "--logflow")
	}
	if debugLogOpts.Tracing != "" {
		sharedArgs = append(sharedArgs, fmt.Sprintf("--tracing=%v", debugLogOpts.Tracing))
	}
	if debugLogOpts.Debug {
		sharedArgs = append(sharedArgs, "--debug")
	}
	return sharedArgs
}

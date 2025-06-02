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

package plugin

type DebugType int

const (
	// DebugTypeProgram indicates that the debug session is for a program.
	DebugTypeProgram DebugType = iota
	// DebugTypePlugin indicates that the debug session is for a plugin.
	DebugTypePlugin
)

type DebugSpec struct {
	// Type is the type of the thing to debug. Can be "program" or "plugin".
	Type DebugType

	// Name is the name of the plugin. Only used if Type is DebugTypePlugin.
	Name string
}

type DebugContext interface {
	// StartDebugging asks the host to start a debug session for the given configuration.
	StartDebugging(info DebuggingInfo) error

	// AttachDebugger returns true if debugging is enabled.
	AttachDebugger(spec DebugSpec) bool
}

type DebuggingInfo struct {
	// Config is the debug configuration (language-specific, see Debug Adapter Protocol)
	Config map[string]interface{}
}

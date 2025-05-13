// Copyright 2016-2024, Pulumi Corporation.
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

package engine

import (
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func newDebugContext(events eventEmitter, attachDebugger []string) plugin.DebugContext {
	return &debugContext{
		attachDebugger: attachDebugger,
		events:         events,
	}
}

type debugContext struct {
	attachDebugger []string     // the debugger types to attach to.
	events         eventEmitter // the channel to emit events into.
}

var _ plugin.DebugContext = (*debugContext)(nil)

func (s *debugContext) StartDebugging(info plugin.DebuggingInfo) error {
	s.events.startDebugging(info)
	return nil
}

func (s *debugContext) AttachDebugger(spec plugin.DebugSpec) bool {
	if slices.Contains(s.attachDebugger, "all") {
		return true
	}
	if spec.Type == plugin.DebugTypeProgram && slices.Contains(s.attachDebugger, "program") {
		return true
	}
	if slices.Contains(s.attachDebugger, "plugins") {
		return true
	}
	for _, requested := range s.attachDebugger {
		if name := strings.TrimPrefix(requested, "plugin="); name == spec.Name {
			return true
		}
	}
	return false
}

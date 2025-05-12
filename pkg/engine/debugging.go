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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func newDebugContext(events eventEmitter) plugin.DebugContext {
	return &debugContext{
		attachDebugger: true,
		events:         events,
	}
}

type debugContext struct {
	attachDebugger bool         // whether debugging is enabled.
	events         eventEmitter // the channel to emit events into.
}

var _ plugin.DebugContext = (*debugContext)(nil)

func (s *debugContext) StartDebugging(info plugin.DebuggingInfo) error {
	s.events.startDebugging(info)
	return nil
}

func (s *debugContext) AttachDebugger() bool {
	return s.attachDebugger
}

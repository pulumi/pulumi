// Copyright 2016-2018, Pulumi Corporation.
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

package display

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func ShowWatchEvents(op string, action apitype.UpdateKind, events <-chan engine.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()
	for e := range events {
		// In the event of cancelation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		// Events ocurring early:
		case engine.PreludeEvent, engine.SummaryEvent, engine.StdoutColorEvent:
			// Ignore it
			continue
		case engine.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload.(engine.DiagEventPayload)
			s := renderDiffDiagEvent(p, opts)
			fprintIgnoreError(os.Stdout, s)
		case engine.ResourcePreEvent:
			p := e.Payload.(engine.ResourcePreEventPayload)
			if shouldShow(p.Metadata, opts) {
				s := fmt.Sprintf("%s %s[%s] %v\n",
					p.Metadata.Op, p.Metadata.URN.Name(), p.Metadata.URN.Type(), p.Metadata.Diffs)
				fprintIgnoreError(os.Stdout, s)
			}
		case engine.ResourceOutputsEvent:
			p := e.Payload.(engine.ResourceOutputsEventPayload)
			if shouldShow(p.Metadata, opts) {
				s := fmt.Sprintf("done %s %s[%s]\n",
					p.Metadata.Op, p.Metadata.URN.Name(), p.Metadata.URN.Type())
				fprintIgnoreError(os.Stdout, s)
			}
		default:
			contract.Failf("unknown event type '%s'", e.Type)
		}
	}
}

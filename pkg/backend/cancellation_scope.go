// Copyright 2016-2023, Pulumi Corporation.
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

package backend

import (
	"context"
	"os"
	"os/signal"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

type cancellationScope struct {
	context *cancel.Context
	sigint  chan os.Signal
	done    chan bool
}

func (s *cancellationScope) Context() *cancel.Context {
	return s.context
}

func (s *cancellationScope) Close() {
	signal.Stop(s.sigint)
	close(s.sigint)
	<-s.done
}

type cancellationScopeSource int

var CancellationScopes = CancellationScopeSource(cancellationScopeSource(0))

func (cancellationScopeSource) NewScope(events chan<- engine.Event, isPreview bool) CancellationScope {
	cancelContext, cancelSource := cancel.NewContext(context.Background())

	c := &cancellationScope{
		context: cancelContext,
		sigint:  make(chan os.Signal),
		done:    make(chan bool),
	}

	go func() {
		for range c.sigint {
			// If we haven't yet received a SIGINT, call the cancellation func. Otherwise call the termination
			// func.
			if cancelContext.CancelErr() == nil {
				message := "^C received; cancelling. If you would like to terminate immediately, press ^C again.\n"
				if !isPreview {
					message += colors.BrightRed + "Note that terminating immediately may lead to orphaned resources " +
						"and other inconsistencies.\n" + colors.Reset
				}
				engine.NewEvent(engine.StdoutColorEvent, engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Cancel()
			} else {
				message := colors.BrightRed + "^C received; terminating" + colors.Reset
				engine.NewEvent(engine.StdoutColorEvent, engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Terminate()
			}
		}
		close(c.done)
	}()
	signal.Notify(c.sigint, os.Interrupt)

	return c
}

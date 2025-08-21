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
	"bufio"
	"bytes"
	"context"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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
		// The channel for signal.Notify should be buffered https://pkg.go.dev/os/signal#Notify
		sigint: make(chan os.Signal, 1),
		done:   make(chan bool),
	}

	go func() {
		for sig := range c.sigint {
			// If we haven't yet received a SIGINT or SIGTERM, call the cancellation func. Otherwise call the
			// termination func.
			if cancelContext.CancelErr() == nil {
				// Grab the stack traces for all goroutines and log them.
				if logging.Verbose >= 9 {
					var b bytes.Buffer
					f := bufio.NewWriter(&b)
					if err := pprof.Lookup("goroutine").WriteTo(f, 2); err != nil {
						logging.V(9).Infof("failed to get goroutine stack traces: %s", err)
					} else {
						if err := f.Flush(); err != nil {
							logging.V(9).Infof("failed to flush buffer: %s", err)
						}
						// We still write out the information we got, even if the flush failed.
						logging.V(9).Infoln("goroutines at time of cancellation:")
						r := bytes.NewReader(b.Bytes())
						scan := bufio.NewScanner(r)
						for scan.Scan() {
							logging.V(9).Info(scan.Text())
						}
					}
				}
				message := "^C received; cancelling. If you would like to terminate immediately, press ^C again.\n"
				if sig == syscall.SIGTERM {
					message = "SIGTERM received; cancelling. If you would like to terminate immediately, send SIGTERM again.\n"
				}
				if !isPreview {
					message += colors.BrightRed + "Note that terminating immediately may lead to orphaned resources " +
						"and other inconsistencies.\n" + colors.Reset
				}
				events <- engine.NewEvent(engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Cancel()
			} else {
				sigdisplay := "^C"
				if sig == syscall.SIGTERM {
					sigdisplay = "SIGTERM"
				}
				message := colors.BrightRed + sigdisplay + " received; terminating" + colors.Reset
				events <- engine.NewEvent(engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Terminate()
			}
		}
		close(c.done)
	}()
	signal.Notify(c.sigint, os.Interrupt, syscall.SIGTERM)

	return c
}

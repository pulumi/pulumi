// Copyright 2025, Pulumi Corporation.
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

//go:build !windows && !js
// +build !windows,!js

package cmdutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func InitPprofServer(ctx context.Context) {
	sigusr := make(chan os.Signal, 1)
	go func() {
		defer logging.Flush()

		<-sigusr

		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			logging.Errorf("could not start listener for pprof server: %s", err)
			return
		}
		mux := http.NewServeMux()
		mux.Handle("/debug/pprof/", http.HandlerFunc(netpprof.Index))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(netpprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(netpprof.Profile))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(netpprof.Symbol))
		mux.Handle("/debug/pprof/trace", http.HandlerFunc(netpprof.Trace))

		serverErr := make(chan error, 1)
		go func() {
			serverErr <- http.Serve(listener, mux) //nolint:gosec // G114
		}()

		u := fmt.Sprintf("http://localhost:%d/debug/pprof/", listener.Addr().(*net.TCPAddr).Port)
		// Don't use logging.V here, we always want to create & write a log file here.
		logging.Infof("pprof server running on %s", u)
		logging.Flush() // Immediately flush after logging the URL so we don't have to wait for the periodic flush.

		select {
		case <-ctx.Done():
		case err := <-serverErr:
			logging.Errorf("pprof server error: %s", err)
		}
		if err := listener.Close(); err != nil {
			logging.Errorf("failed to close pprof listener: %s", err)
		}
	}()
	signal.Notify(sigusr, syscall.SIGUSR1)
}

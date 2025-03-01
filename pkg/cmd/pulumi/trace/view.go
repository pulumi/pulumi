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

package trace

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/pulumi/appdash"
	"github.com/pulumi/appdash/traceapp"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func NewViewTraceCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "view-trace [trace-file]",
		Short: "Display a trace from the Pulumi CLI",
		Long: "Display a trace from the Pulumi CLI.\n" +
			"\n" +
			"This command is used to display execution traces collected by a prior\n" +
			"invocation of the Pulumi CLI.\n" +
			"\n" +
			"This command loads trace data from the indicated file and starts a\n" +
			"webserver to display the trace. By default, this server will listen\n" +
			"port 8008; the --port flag can be used to change this if necessary.",
		Args:   cmdutil.ExactArgs(1),
		Hidden: !env.DebugCommands.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				return err
			}

			store := appdash.NewMemoryStore()
			if err := readTrace(args[0], store); err != nil {
				return err
			}

			app, err := traceapp.New(nil, url)
			if err != nil {
				return err
			}
			app.Store, app.Queryer = store, store

			fmt.Printf("Displaying trace at %v\n", url)
			return http.ListenAndServe(fmt.Sprintf(":%d", port), app) //nolint:gosec
		},
	}

	cmd.PersistentFlags().IntVar(&port, "port", 8008,
		"the port the trace viewer will listen on")

	return cmd
}

// Copyright 2026, Pulumi Corporation.
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

package needle

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func RequireBackend(backend *backend.Backend) Stitch {
	return &request{
		value:       requireBackend,
		fulfillInto: func(s *state) { *backend = s.backend },
	}
}

var requireBackend = &value{
	deps: []*value{optionProject},
	get: func(cmd *cobra.Command, state *state, _ any) error {
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(state.WS, state.Env, state.project)
		if err != nil {
			return fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))
		insecure := pkgWorkspace.GetCloudInsecure(state.WS, url)

		// Only set current if we don't currently have a cloud URL set.
		b, err := state.LM.Login(
			cmd.Context(), state.WS, state.DiagSink, url, state.project, url == "", insecure, state.Color)
		if err != nil {
			return err
		}
		state.backend = b
		return nil
	},
}

var optionBackend = &value{
	deps: []*value{optionProject},
	get: func(cmd *cobra.Command, state *state, _ any) error {
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(state.WS, state.Env, state.project)
		if err != nil {
			return fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))

		// Only set current if we don't currently have a cloud URL set.
		b, err := state.LM.Current(cmd.Context(), state.WS, state.DiagSink, url, state.project, url == "")
		if errors.Is(err, backenderr.ErrLoginRequired) {
			b, err = nil, nil
		}
		if err != nil {
			return err
		}
		state.backend = b
		return nil
	},
}

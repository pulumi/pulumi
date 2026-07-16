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

package adder

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// Backend resolves the current backend, logging the user in if necessary.
func (e Environment) Backend(cmd *cobra.Command) (backend.Backend, error) {
	return bagFrom(cmd).loginBackend.get(func() (backend.Backend, error) {
		e := e.defaults(cmd)
		project, _, err := e.Project(cmd)
		if err != nil {
			return nil, err
		}
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(e.WS, e.Env, project)
		if err != nil {
			return nil, fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))
		insecure := pkgWorkspace.GetCloudInsecure(e.WS, url)

		// Only set current if we don't currently have a cloud URL set.
		return e.LM.Login(
			cmd.Context(), e.WS, e.DiagSink, url, project, url == "", insecure, e.Color)
	})
}

// CurrentBackend resolves the backend the user is currently logged in to. It
// returns a nil backend (and no error) when the user isn't logged in.
func (e Environment) CurrentBackend(cmd *cobra.Command) (backend.Backend, error) {
	return bagFrom(cmd).currentBackend.get(func() (backend.Backend, error) {
		e := e.defaults(cmd)
		project, _, err := e.Project(cmd)
		if err != nil {
			return nil, err
		}
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(e.WS, e.Env, project)
		if err != nil {
			return nil, fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))

		b, err := e.LM.Current(cmd.Context(), e.WS, e.DiagSink, url, project, url == "")
		if errors.Is(err, backenderr.ErrLoginRequired) {
			return nil, nil
		}
		return b, err
	})
}

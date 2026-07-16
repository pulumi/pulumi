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
func (s Spindle) Backend(cmd *cobra.Command) (backend.Backend, error) {
	return bagFrom(cmd).loginBackend.get(func() (backend.Backend, error) {
		s := s.defaults(cmd)
		project, _, err := s.Project(cmd)
		if err != nil {
			return nil, err
		}
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(s.WS, s.Env, project)
		if err != nil {
			return nil, fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))
		insecure := pkgWorkspace.GetCloudInsecure(s.WS, url)

		// Only set current if we don't currently have a cloud URL set.
		return s.LM.Login(
			cmd.Context(), s.WS, s.DiagSink, url, project, url == "", insecure, s.Color)
	})
}

// CurrentBackend resolves the backend the user is currently logged in to. It
// returns a nil backend (and no error) when the user isn't logged in.
func (s Spindle) CurrentBackend(cmd *cobra.Command) (backend.Backend, error) {
	return bagFrom(cmd).currentBackend.get(func() (backend.Backend, error) {
		s := s.defaults(cmd)
		project, _, err := s.Project(cmd)
		if err != nil {
			return nil, err
		}
		url, err := pkgWorkspace.GetCurrentCloudURLWithAgentFallback(s.WS, s.Env, project)
		if err != nil {
			return nil, fmt.Errorf("could not get cloud url: %w", err)
		}
		slog.Info("Current cloud URL", slog.String("url", url))

		b, err := s.LM.Current(cmd.Context(), s.WS, s.DiagSink, url, project, url == "")
		if errors.Is(err, backenderr.ErrLoginRequired) {
			return nil, nil
		}
		return b, err
	})
}

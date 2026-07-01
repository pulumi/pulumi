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

package config

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

// A linked environment reference has the form "<project>/<name>" and may carry a version qualifier on
// the tail as "@<version>" or ":<version>". ESC accepts both separators and treats "@" as primary with
// ":" as the fallback (esc cmd/esc/cli/env.go), so the helpers below must too — detecting only "@"
// silently misses a ":"-pinned ref.

// envRefVersion returns the pinned revision or tag in an environment reference, or "" when unpinned.
func envRefVersion(ref string) string {
	if _, version, found := strings.Cut(ref, "@"); found {
		return version
	}
	_, version, _ := strings.Cut(ref, ":")
	return version
}

// stripEnvVersion returns the bare "<project>/<name>" form with any version qualifier removed.
func stripEnvVersion(ref string) string {
	if base, _, found := strings.Cut(ref, "@"); found {
		return base
	}
	base, _, _ := strings.Cut(ref, ":")
	return base
}

func splitEnvRef(ref string) (project, name string, err error) {
	project, name, found := strings.Cut(stripEnvVersion(ref), "/")
	if !found || project == "" || name == "" {
		return "", "", fmt.Errorf("malformed environment reference %q; expected <project>/<name>", ref)
	}
	return project, name, nil
}

// rejectIfPinned refuses a mutation when the stack's remote configuration is pinned to a specific
// environment revision or tag. Writing through a pinned ref would silently target the latest revision
// the pinned stack never resolves, so callers must unpin first. An explicit --config-file routes the
// write to a local file regardless of the stack's remote link, so the pin guard does not apply.
func rejectIfPinned(stack backend.Stack, configFile string) error {
	loc := stack.ConfigLocation()
	if !configStoreIsRemote(stack, configFile) || loc.EscEnv == nil {
		return nil
	}
	if envRefVersion(*loc.EscEnv) != "" {
		return fmt.Errorf("the stack's configuration is pinned to %s; "+
			"run `pulumi config env pin latest` to unpin before making changes", *loc.EscEnv)
	}
	return nil
}

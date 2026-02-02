// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

const defaultESCProject = "insights"

// generateESCEnvironmentYAML generates the ESC environment YAML for AWS OIDC login.
func generateESCEnvironmentYAML(roleARN string) []byte {
	yaml := fmt.Sprintf(`values:
  aws:
    login:
      fn::open::aws-login:
        oidc:
          duration: 1h
          roleArn: %s
          sessionName: pulumi-insights-discovery
`, roleARN)

	return []byte(yaml)
}

// createESCEnvironment creates an ESC environment with the given OIDC configuration.
func createESCEnvironment(
	ctx context.Context,
	envBackend backend.EnvironmentsBackend,
	orgName string,
	projectName string,
	envName string,
	roleARN string,
) error {
	yaml := generateESCEnvironmentYAML(roleARN)

	diags, err := envBackend.CreateEnvironment(ctx, orgName, projectName, envName, yaml)
	if err != nil {
		return fmt.Errorf("creating ESC environment %s/%s/%s: %w", orgName, projectName, envName, err)
	}
	if len(diags) != 0 {
		return fmt.Errorf("ESC environment validation failed: %s", diags)
	}

	return nil
}

// escEnvironmentRef builds a formatted ESC environment reference string.
func escEnvironmentRef(orgName, projectName, envName string) string {
	return fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)
}

// parseESCEnvironmentRef parses an ESC environment reference in the format
// "project/env" or "org/project/env". Returns orgName, projectName, envName.
func parseESCEnvironmentRef(ref string, defaultOrg string) (string, string, string, error) {
	// Strip @version suffix if present
	ref, _, _ = strings.Cut(ref, "@")

	parts := strings.Split(ref, "/")
	switch len(parts) {
	case 2:
		// project/env
		return defaultOrg, parts[0], parts[1], nil
	case 3:
		// org/project/env
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid ESC environment reference %q: expected format project/env or org/project/env", ref)
	}
}

// defaultESCEnvName returns the default ESC environment name for an AWS account.
func defaultESCEnvName(accountID string) string {
	return fmt.Sprintf("aws-%s", accountID)
}

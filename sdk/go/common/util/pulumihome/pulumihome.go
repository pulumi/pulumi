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

// Package pulumihome resolves the per-user Pulumi directory (PULUMI_HOME,
// defaulting to '<user's home>/.pulumi').
package pulumihome

import (
	"fmt"
	"os/user"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

// BookkeepingDir is the name of the Pulumi bookkeeping folder (like .git for git).
const BookkeepingDir = ".pulumi"

// Dir returns the path of the '.pulumi' folder where Pulumi puts its artifacts.
func Dir() (string, error) {
	// Allow the folder we use to be overridden by an environment variable
	dir := env.Home.Value()
	if dir != "" {
		return dir, nil
	}

	// Otherwise, use the current user's home dir + .pulumi
	user, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("getting current user: %w", err)
	}

	if user == nil || user.HomeDir == "" {
		return "", fmt.Errorf("could not find user home directory, set %s", env.Home.Var().Name())
	}

	return filepath.Join(user.HomeDir, BookkeepingDir), nil
}

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

package plugin

import (
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CommandRemediation is a command that a provider suggests the user run to resolve a problem it reported in
// CheckConfig, such as missing or expired credentials. Callers must show the command to the user and get their consent
// before running it.
type CommandRemediation struct {
	// Description is a short, imperative summary of what the command does, such as "Sign in to AWS".
	Description string
	// Command is the program, looked up on PATH, and its arguments. It is run without a shell.
	Command []string
	// Env holds environment variables to set for the command, in addition to the caller's environment.
	Env map[string]string
}

// remediationsFromRPC converts the remediations of a CheckConfig2 failure received from a provider. Remediations
// without a command to run are dropped here, so the rest of the engine can assume every remediation has one.
func remediationsFromRPC(rpcRemediations []*pulumirpc.CommandRemediation) []CommandRemediation {
	var remediations []CommandRemediation
	for _, r := range rpcRemediations {
		if len(r.GetCommand()) == 0 || r.GetCommand()[0] == "" {
			continue
		}
		remediations = append(remediations, CommandRemediation{
			Description: r.GetDescription(),
			Command:     r.GetCommand(),
			Env:         r.GetEnv(),
		})
	}
	return remediations
}

func remediationsToRPC(remediations []CommandRemediation) []*pulumirpc.CommandRemediation {
	rpcRemediations := make([]*pulumirpc.CommandRemediation, 0, len(remediations))
	for _, r := range remediations {
		rpcRemediations = append(rpcRemediations, &pulumirpc.CommandRemediation{
			Description: r.Description,
			Command:     r.Command,
			Env:         r.Env,
		})
	}
	return rpcRemediations
}

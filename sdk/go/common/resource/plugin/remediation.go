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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CommandRemediation is a command that a provider suggests the user run to resolve a problem it reported in
// CheckConfig, such as missing or expired credentials. Callers must show the command to the user and get their consent
// before running it.
type CommandRemediation struct {
	// Description is a short, imperative summary of what the command does, such as "Sign in to AWS".
	Description string
	// Command is the program and its arguments. The program is looked up on PATH and run without a shell, attached
	// to the user's terminal.
	Command []string
	// Env holds environment variables to set for the command, in addition to the caller's environment.
	Env map[string]string
}

// checkConfigFailureFromRPC converts a CheckConfig2 failure received from a provider. Remediations without a command
// to run are dropped here, so the rest of the engine can assume every remediation has one.
func checkConfigFailureFromRPC(failure *pulumirpc.CheckConfigFailure) CheckConfigFailure {
	var remediations []CommandRemediation
	for _, r := range failure.GetRemediations() {
		if len(r.GetCommand()) == 0 || r.GetCommand()[0] == "" {
			continue
		}
		remediations = append(remediations, CommandRemediation{
			Description: r.GetDescription(),
			Command:     r.GetCommand(),
			Env:         r.GetEnv(),
		})
	}
	return CheckConfigFailure{
		Property:     resource.PropertyKey(failure.GetProperty()),
		Reason:       failure.GetReason(),
		Remediations: remediations,
		Hint:         hintFromRPC(failure.GetHint()),
		HintMessage:  failure.GetHintMessage(),
	}
}

// checkConfigFailureToRPC converts a failure to send to the engine in a CheckConfig2 response.
func checkConfigFailureToRPC(failure CheckConfigFailure) *pulumirpc.CheckConfigFailure {
	remediations := make([]*pulumirpc.CommandRemediation, 0, len(failure.Remediations))
	for _, r := range failure.Remediations {
		remediations = append(remediations, &pulumirpc.CommandRemediation{
			Description: r.Description,
			Command:     r.Command,
			Env:         r.Env,
		})
	}
	return &pulumirpc.CheckConfigFailure{
		Property:     string(failure.Property),
		Reason:       failure.Reason,
		Remediations: remediations,
		Hint:         failure.Hint.rpc(),
		HintMessage:  failure.HintMessage,
	}
}

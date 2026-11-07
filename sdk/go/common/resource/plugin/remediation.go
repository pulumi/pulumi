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
	"errors"
	"maps"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CommandRemediation is a command that a provider suggests the user run to resolve a problem it reported, such as
// missing or expired credentials. Callers must show the command to the user and get their consent before running it.
type CommandRemediation struct {
	// Description is a short, imperative summary of what the command does, such as "Sign in to AWS".
	Description string
	// Command is the program and its arguments. The program is looked up on PATH and run without a shell.
	Command []string
	// Env holds environment variables to set for the command, in addition to the caller's environment.
	Env map[string]string
	// Interactive is true if the command needs the user's terminal.
	Interactive bool
}

// ConfigureErrorRemediations returns the remediations that a provider attached to an error returned by Configure.
func ConfigureErrorRemediations(err error) []CommandRemediation {
	rpcErr, ok := errors.AsType[*rpcerror.Error](err)
	if !ok {
		if rpcErr, ok = rpcerror.FromError(err); !ok {
			return nil
		}
	}
	var remediations []CommandRemediation
	for _, detail := range rpcErr.Details() {
		if r, ok := detail.(*pulumirpc.CommandRemediation); ok {
			if remediation, ok := remediationFromRPC(r); ok {
				remediations = append(remediations, remediation)
			}
		}
	}
	return remediations
}

func checkFailureFromRPC(failure *pulumirpc.CheckFailure) CheckFailure {
	var remediations []CommandRemediation
	for _, r := range failure.GetRemediations() {
		if remediation, ok := remediationFromRPC(r); ok {
			remediations = append(remediations, remediation)
		}
	}
	return CheckFailure{
		Property:     resource.PropertyKey(failure.GetProperty()),
		Reason:       failure.GetReason(),
		Remediations: remediations,
	}
}

func checkFailureToRPC(failure CheckFailure) *pulumirpc.CheckFailure {
	remediations := make([]*pulumirpc.CommandRemediation, 0, len(failure.Remediations))
	for _, r := range failure.Remediations {
		remediations = append(remediations, &pulumirpc.CommandRemediation{
			Description: r.Description,
			Command:     r.Command,
			Env:         r.Env,
			Interactive: r.Interactive,
		})
	}
	return &pulumirpc.CheckFailure{
		Property:     string(failure.Property),
		Reason:       failure.Reason,
		Remediations: remediations,
	}
}

// remediationFromRPC converts a remediation, dropping it if it has no command to run.
func remediationFromRPC(r *pulumirpc.CommandRemediation) (CommandRemediation, bool) {
	if len(r.GetCommand()) == 0 || r.GetCommand()[0] == "" {
		return CommandRemediation{}, false
	}
	return CommandRemediation{
		Description: r.GetDescription(),
		Command:     r.GetCommand(),
		Env:         maps.Clone(r.GetEnv()),
		Interactive: r.GetInteractive(),
	}, true
}

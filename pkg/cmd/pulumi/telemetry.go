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

package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/telemetry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func invocationTelemetry(c *cobra.Command) []telemetry.KV {
	return []telemetry.KV{
		telemetry.Command(c),
		telemetry.Flags(c),
		telemetry.EnvVars(env.Global),
		telemetry.CI(ciutil.IsCI()),
		telemetry.CISystem(ciutil.DetectVars().Name),
		telemetry.Interactive(cmdutil.Interactive()),
		telemetry.AutomationAPI(cmdutil.IsTruthy(os.Getenv("PULUMI_AUTOMATION_API"))),
		telemetry.UsingAI(os.Getenv),
		telemetry.OS(),
		telemetry.Arch(),
	}
}

func exitTelemetry(ctx context.Context, err error) []telemetry.KV {
	facts := []telemetry.KV{telemetry.ExitCode(cmd.ExitCodeFor(err))}
	if start, ok := cmdutil.ProcessStartTimeFromContext(ctx); ok {
		facts = append(facts, telemetry.Duration(time.Since(start)))
	}
	return facts
}

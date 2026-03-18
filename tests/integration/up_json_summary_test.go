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

package ints

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	ui "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

//nolint:paralleltest // uses real pulumi binary and mutates env/backend
func TestUp_JSONSummaryFooter(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.WriteTestFile("Pulumi.yaml", `
name: up-json-summary-test
runtime: nodejs
`)

	e.WriteTestFile("index.ts", `
import * as pulumi from "@pulumi/pulumi";

const cfg = new pulumi.Config();
const value = cfg.get("value") || "default";

export const output = value;
`)

	// Use local file backend to avoid service dependency.
	e.Backend = e.LocalURL()

	// Install NodeJS dependencies for the test program.
	{
		cmd := e.SetupCommandIn(context.Background(), e.CWD, "npm", "install", "@pulumi/pulumi")
		err := cmd.Run()
		require.NoError(t, err)
	}

	// Run `pulumi up --json --yes --non-interactive` and capture stdout.
	cmd := e.SetupCommandIn(
		context.Background(),
		e.CWD,
		"pulumi",
		"up",
		"--yes",
		"--json",
		"--non-interactive",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err := cmd.Run()
	require.NoError(t, err)

	// Parse stdout as JSONL and inspect the last non-empty line as the summary.
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lastLine = line
	}
	require.NoError(t, scanner.Err())
	require.NotEmpty(t, lastLine, "expected at least one JSON line in output")

	var summary ui.OperationSummaryJSON

	err = json.Unmarshal([]byte(lastLine), &summary)
	require.NoError(t, err, "last line should be valid JSON summary")

	require.Equal(t, ui.OperationResultSucceeded, summary.Result)
	require.NotEmpty(t, summary.ChangeSummary)
	// Duration should be a non-empty string like "1.234s".
	require.NotEmpty(t, summary.Duration)
}


// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// regresion test for https://github.com/pulumi/pulumi/issues/18659
//
//nolint:paralleltest // sets PULUMI_HOME and os.Args
func TestNewHelp(t *testing.T) {
	tempdir := t.TempDir()
	t.Setenv("PULUMI_HOME", tempdir)

	args := os.Args
	os.Args = []string{"pulumi", "help", "new"}
	defer func() { os.Args = args }()

	cmd, _ := NewPulumiCmd()
	err := cmd.Execute()
	require.NoError(t, err)
}

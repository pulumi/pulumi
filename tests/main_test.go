// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/backend/local"
)

func TestMain(m *testing.M) {
	// Disable stack backups for tests to avoid filling up ~/.pulumi/backups with unnecessary
	// backups of test stacks.
	if err := os.Setenv(local.DisableCheckpointBackupsEnvVar, "1"); err != nil {
		fmt.Printf("error setting env var '%s': %v\n", local.DisableCheckpointBackupsEnvVar, err)
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}

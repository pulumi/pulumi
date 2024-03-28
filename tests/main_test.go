// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

func TestMain(m *testing.M) {
	// Disable stack backups for tests to avoid filling up ~/.pulumi/backups with unnecessary
	// backups of test stacks.
	disableCheckpointBackups := env.DIYBackendDisableCheckpointBackups.Var().Name()
	if err := os.Setenv(disableCheckpointBackups, "1"); err != nil {
		fmt.Printf("error setting env var '%s': %v\n", disableCheckpointBackups, err)
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}

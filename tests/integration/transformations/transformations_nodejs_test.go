// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build nodejs all

package ints

import (
	"github.com/pulumi/pulumi/pkg/v2/testing/integration"
	"path/filepath"
)

var dirs = []string{
	"simple",
}

func TestNodejsTransformations(t *testing.T) {
	for _, dir := range dirs {
		d := filepath.Join("nodejs", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:                    d,
				Dependencies:           []string{"@pulumi/pulumi"},
				Quick:                  true,
				ExtraRuntimeValidation: Validator("nodejs"),
			})
		})
	}
}



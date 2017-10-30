// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows
package cmd

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows/registry"
)

func TestGetCredsFilePath(t *testing.T) {

	want := registry.ExpandEnv(localAppData)
	want = path.Join(path.Join(want, pulumiAppName), pulumiSettingsFolder)

	got, err := getCredsFilePath()
	if err != nil {
		assert.Fail(t, "getCredsFilePath failed with error %v", err)
		return
	}

	got, _ = filepath.Split(got)

	got, err = filepath.Abs(got)
	if !assert.NoError(t, err, "filePath.Abs failed with error %v ", err) {
		return
	}

	assert.Equal(t, want, got)
}

// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmd

import (
	"os/user"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCredsFilePath(t *testing.T) {

	user, err := user.Current()
	if user == nil || err != nil {
		return
	}
	want := path.Join(user.HomeDir, pulumiSettingsFolder)

	got, err := getCredsFilePath()
	if err != nil {
		assert.Fail(t, "getCredsFilePath Failed")
		return
	}

	got, _ = filepath.Split(got)
	got, _ = filepath.Abs(got)
	assert.Equal(t, want, got)
}

// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path"
)

// getCredsFilePath returns the path to the Pulumi credentials file on disk, if it
// exists. Otherwise nil and the related OS error.
func getCredsFilePath() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", fmt.Errorf("getting creds file path: failed to get current user")
	}

	pulumiFolder := path.Join(user.HomeDir, pulumiSettingsFolder)

	err = os.MkdirAll(pulumiFolder, permUserAllRestNone)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s'", pulumiFolder)
	}

	return path.Join(pulumiFolder, pulumiCredentialsFileName), nil
}

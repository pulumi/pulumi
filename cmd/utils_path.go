// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build !windows

package cmd

import (
	"fmt"
	"os/user"
)

// getCredsFilePath returns the path to the Pulumi credentials file would be on disk and returns
// any OS rellated error. It doesnt guarantee if the file actually exists
func getCredsFileRoot() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", fmt.Errorf("getting creds file path: failed to get current user")
	}

	return user.HomeDir, nil
}

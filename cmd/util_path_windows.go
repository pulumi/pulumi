// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows
package cmd

import (
	"fmt"
	"os"
	"path"
)

// getCredsFilePath returns the path to the Pulumi credentials file on disk, if it
// exists. Otherwise nil and the related OS error.
func getCredsFilePath() (string, error) {
	appData := registry.ExpandString("%APPDATA")

	pulumiFolder := path.Join(appData, pulumiSettingsFolder)

	err = os.MkdirAll(pulumiFolder, permUserAllRestNone)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s'", pulumiFolder)
	}

	return path.Join(pulumiFolder, pulumiCredentialsFileName), nil
}

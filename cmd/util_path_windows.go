// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows
package cmd

import (
	"fmt"
	"os"
	"path"
)

var localAppData = "%LOCALAPPDATA"
var pulumiAppName = "pulumi"

// getCredsFilePath returns the path to the Pulumi credentials file on disk, if it
// exists. Otherwise nil and the related OS error.
func getCredsFilePath() (string, error) {

	// get the local appdata folder for Windows
	// %users%\AppData\Local\
	appData := registry.ExpandString(localAppData)

	// .Pulumi directory will be under %LOCALAPPDATA%\%pulumiAppName%
	pulumiFolder := path.Join(path.Join(appData, pulumiAppName), pulumiSettingsFolder)

	err = os.MkdirAll(pulumiFolder, permUserAllRestNone)

	if err != nil {
		return "", fmt.Errorf("failed to create '%s'", pulumiFolder)
	}

	return path.Join(pulumiFolder, pulumiCredentialsFileName), nil
}

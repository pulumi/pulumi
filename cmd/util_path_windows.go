// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
// +build windows

package cmd

import (
	"os"
	"path"
)

// var localAppData = "%LOCALAPPDATA%"
var pulumiAppName = "pulumi"

// getCredsFilePath returns the path to the Pulumi credentials file would be on disk and returns
// any OS rellated error. It doesnt guarantee if the file actually exists
func getCredsFileRoot() (string, error) {

	// get the local appdata folder for Windows
	// %users%\AppData\Local\
	appData := os.ExpandEnv("$(LOCALAPPDATA)")

	return path.Join(appData, pulumiAppName)
}

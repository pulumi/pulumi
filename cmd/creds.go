// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"

	"github.com/pkg/errors"
)

// pulumiSettingsFolder is the name of the folder we put in the user's home dir to store settings.
// TODO(pulumi/pulumi-service#49): Return this from a function that takes OS-idioms into account.
const pulumiSettingsFolder = ".pulumi"

// permUserAllRestNone defines the file permissions that the
// user has RWX access, and group and other have no access.
const permUserAllRestNone = 0600

// accountCredentials hold the information necessary for authenticating Pulumi Cloud API requests.
type accountCredentials struct {
	AccessToken string `json:"accessToken"`
}

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

	return path.Join(pulumiFolder, "credentials.json"), nil
}

// errCredsNotFound is the error returned if the credentials file is not found.
var errCredsNotFound = errors.New("credentials file not found")

// getStoredCredentials returns any credentials stored on the local machine. Returns any
// IO error if found. errCredsNotFound if no credentials file is present.
func getStoredCredentials() (accountCredentials, error) {
	var creds accountCredentials

	credsFile, err := getCredsFilePath()
	if err != nil {
		return creds, err
	}

	// Creds file does not exist.
	if _, err = os.Stat(credsFile); os.IsNotExist(err) {
		return creds, errCredsNotFound
	}

	c, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return creds, fmt.Errorf("reading '%s': %v", credsFile, err)
	}

	if err = json.Unmarshal(c, &creds); err != nil {
		return creds, fmt.Errorf("unmarshalling credentials file: %v", err)
	}
	return creds, nil
}

// storeCredentials updates the stored credentials on the machine, replacing the
// existing set.
func storeCredentials(creds accountCredentials) error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	raw, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshalling credentials object: %v", err)
	}
	return ioutil.WriteFile(credsFile, raw, permUserAllRestNone)
}

// deleteStoredCredentials deletes the user's stored credentials.
func deleteStoredCredentials() error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	_, err = os.Stat(credsFile)
	if os.IsNotExist(err) {
		return nil
	}

	return os.Remove(credsFile)
}

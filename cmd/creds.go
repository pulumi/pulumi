// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
)

// pulumiSettingsFolder is the name of the folder we put in the user's home dir to store settings.
// TODO(pulumi/pulumi-service#49): Return this from a function that takes OS-idioms into account.
const pulumiSettingsFolder = ".pulumi"

// permUserAllRestNone defines the file permissions that the
// user has RWX access, and group and other have no access.
const permUserAllRestNone = 0700

// AccountCredentials hold the information necessary for authenticating Pulumi Cloud API requests.
type AccountCredentials struct {
	GitHubLogin string `json:"githubLogin"`
	AccessToken string `json:"token"`
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

// GetStoredCredentials returns any credentials stored on the local machine. nil if
// they are not present. Returns any IO errors which occur.
func GetStoredCredentials() (*AccountCredentials, error) {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return nil, err
	}

	// Creds file does not exist.
	if _, err = os.Stat(credsFile); os.IsNotExist(err) {
		return nil, nil
	}

	c, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return nil, fmt.Errorf("reading '%s': %v", credsFile, err)
	}

	var accountCreds AccountCredentials
	if err = json.Unmarshal(c, &accountCreds); err != nil {
		return nil, fmt.Errorf("unmarshalling credentials file: %v", err)
	}
	return &accountCreds, nil
}

// StoreCredentials updates the stored credentials on the machine, replacing the
// existing set.
func StoreCredentials(creds AccountCredentials) error {
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

// DeleteStoredCredentials deletes the user's stored credentials.
func DeleteStoredCredentials() error {
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

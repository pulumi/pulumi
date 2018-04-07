// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/pkg/errors"
)

// PulumiCredentialsPathEnvVar is a path to the folder where credentials are stored.
// We use this in testing so that tests which log in and out do not impact the local developer's
// credentials or tests interacting with one another
const PulumiCredentialsPathEnvVar = "PULUMI_CREDENTIALS_PATH"

// GetAccessToken returns an access token underneath a given key.
func GetAccessToken(key string) (string, error) {
	creds, err := GetStoredCredentials()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if creds.AccessTokens == nil {
		return "", nil
	}
	return creds.AccessTokens[key], nil
}

// DeleteAccessToken deletes an access token underneath the given key.
func DeleteAccessToken(key string) error {
	creds, err := GetStoredCredentials()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if creds.AccessTokens != nil {
		delete(creds.AccessTokens, key)
	}
	if creds.Current == key {
		creds.Current = ""
	}
	return StoreCredentials(creds)
}

// StoreAccessToken saves the given access token underneath the given key.
func StoreAccessToken(key string, token string, current bool) error {
	creds, err := GetStoredCredentials()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if creds.AccessTokens == nil {
		creds.AccessTokens = make(map[string]string)
	}
	creds.AccessTokens[key] = token
	if current {
		creds.Current = key
	}
	return StoreCredentials(creds)
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.  It contains
// a map from the cloud API URL to the associated access token.
type Credentials struct {
	Current      string            `json:"current,omitempty"`      // the currently selected key.
	AccessTokens map[string]string `json:"accessTokens,omitempty"` // a map of arbitrary key strings to tokens.
}

// getCredsFilePath returns the path to the Pulumi credentials file on disk, regardless of
// whether it exists or not.
func getCredsFilePath() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", errors.Wrapf(err, "getting creds file path: failed to get current user")
	}

	// Allow the folder we use to store credentials to be overridden by tests
	pulumiFolder := os.Getenv(PulumiCredentialsPathEnvVar)
	if pulumiFolder == "" {
		pulumiFolder = filepath.Join(user.HomeDir, BookkeepingDir)
	}

	err = os.MkdirAll(pulumiFolder, 0700)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create '%s'", pulumiFolder)
	}

	return filepath.Join(pulumiFolder, "credentials.json"), nil
}

// GetCurrentCloudURL returns the URL of the cloud we are currently connected to. This may be empty if we
// have not logged in.
func GetCurrentCloudURL() (string, error) {
	creds, err := GetStoredCredentials()
	if err != nil {
		return "", err
	}

	return creds.Current, nil
}

// GetStoredCredentials returns any credentials stored on the local machine.
func GetStoredCredentials() (Credentials, error) {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return Credentials{}, err
	}

	c, err := ioutil.ReadFile(credsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, nil
		}
		return Credentials{}, errors.Wrapf(err, "reading '%s'", credsFile)
	}

	var creds Credentials
	if err = json.Unmarshal(c, &creds); err != nil {
		return Credentials{}, errors.Wrapf(err, "unmarshalling credentials file")
	}
	return creds, nil
}

// StoreCredentials updates the stored credentials on the machine, replacing the existing set.  If the credentials
// are empty, the auth file will be deleted rather than just serializing an empty map.
func StoreCredentials(creds Credentials) error {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return err
	}

	if len(creds.AccessTokens) == 0 {
		err = os.Remove(credsFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	raw, err := json.MarshalIndent(creds, "", "    ")
	if err != nil {
		return errors.Wrapf(err, "marshalling credentials object")
	}
	return ioutil.WriteFile(credsFile, raw, 0600)
}

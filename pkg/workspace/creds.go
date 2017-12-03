// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"

	"github.com/pkg/errors"
)

// UseAltCredentialsLocationEnvVar is the name of an environment variable which if set, will cause
// pulumi to use an alternative file path for saving and updating user credentials. This allows for
// a script or testcase to login/logout without impacting regular usage.
const UseAltCredentialsLocationEnvVar = "PULUMI_USE_ALT_CREDENTIALS_LOCATION"

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
	return StoreCredentials(creds)
}

// StoreAccessToken saves the given access token underneath the given key.
func StoreAccessToken(key string, token string) error {
	creds, err := GetStoredCredentials()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if creds.AccessTokens == nil {
		creds.AccessTokens = make(map[string]string)
	}
	creds.AccessTokens[key] = token
	return StoreCredentials(creds)
}

// Credentials hold the information necessary for authenticating Pulumi Cloud API requests.  It contains
// a map from the cloud API URL to the associated access token.
type Credentials struct {
	AccessTokens map[string]string `json:"accessTokens"`
}

// getCredsFilePath returns the path to the Pulumi credentials file on disk, regardless of
// whether it exists or not.
func getCredsFilePath() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", fmt.Errorf("getting creds file path: failed to get current user")
	}

	pulumiFolder := path.Join(user.HomeDir, BookkeepingDir)
	err = os.MkdirAll(pulumiFolder, 0700)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s'", pulumiFolder)
	}

	// If we are running as part of unit tests, we want to save/restore a different set
	// of credentials as to not modify the developer's machine.
	credentialsFile := "credentials.json"
	if os.Getenv(UseAltCredentialsLocationEnvVar) != "" {
		credentialsFile = "alt-credentials.json"
	}

	return path.Join(pulumiFolder, credentialsFile), nil
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
		return Credentials{}, errors.Wrapf(err, "reading '%s': %v", credsFile)
	}

	var creds Credentials
	if err = json.Unmarshal(c, &creds); err != nil {
		return Credentials{}, fmt.Errorf("unmarshalling credentials file: %v", err)
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
		return fmt.Errorf("marshalling credentials object: %v", err)
	}
	return ioutil.WriteFile(credsFile, raw, 0600)
}

// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// PulumiAccessTokenEnvVar is the environment variable used to bypass a prompt on login.
const PulumiAccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log into the Pulumi Cloud Console",
		Long:  "Log into the Pulumi Cloud Console. You can script by using PULUMI_ACCESS_TOKEN environment variable.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return loginCmd()
		}),
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of the Pulumi CLI",
		Long:  "Log out of the Pulumi CLI. Deletes stored credentials on the local machine.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return deleteStoredCredentials()
		}),
	}
}

// loginCmd is the implementation of the login command.
func loginCmd() error {
	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(PulumiAccessTokenEnvVar)
	if accessToken != "" {
		fmt.Printf("Using access token from %s.\n", PulumiAccessTokenEnvVar)
	} else {
		token, err := readConsole("Enter Pulumi access token")
		if err != nil {
			return err
		}
		accessToken = token
	}

	// Try and use the credentials to see if they are valid.
	valid, err := isValidAccessToken(accessToken)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid access token")
	}

	// Save them.
	creds := accountCredentials{
		AccessToken: accessToken,
	}
	return storeCredentials(creds)
}

// isValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func isValidAccessToken(accessToken string) (bool, error) {
	// Make a request to get the authenticated user. If it returns a successful result, the token
	// checks out.
	if err := pulumiRESTCallWithAccessToken("GET", "/user", nil, nil, accessToken); err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, nil
		}
		return false, fmt.Errorf("testing access token: %v", err)
	}
	return true, nil
}

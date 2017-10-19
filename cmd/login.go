// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi-service/pkg/apitype"
	"github.com/pulumi/pulumi/cmd/cloud"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log into the Pulumi CLI",
		Long:  "Log into the Pulumi CLI. You can script by using PULUMI_ACCESS_TOKEN environment variable.",
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
			return cloud.DeleteStoredCredentials()
		}),
	}
}

// loginCmd is the implementation of the login command.
func loginCmd() error {
	// Check if the the user is already logged in.
	storedCreds, err := cloud.GetStoredCredentials()
	if storedCreds != nil && err == nil {
		return fmt.Errorf("already logged in")
	}

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	//
	// However, if PULUMI_ACCESS_TOKEN is available, we'll use that.
	accessToken := os.Getenv("PULUMI_ACCESS_TOKEN")
	if accessToken != "" {
		fmt.Println("Using access token from PULUMI_ACCESS_TOKEN.")
	} else {
		fmt.Println("Enter Pulumi access token:")
		reader := bufio.NewReader(os.Stdin)
		raw, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading STDIN: %v", err)
		}
		accessToken = strings.TrimSpace(raw)
	}

	// We don't know if the access token is valid or not. So we'll use it and see if it checks out.
	// Store the credentials and try to make an authenticated request to look up the user.
	creds := cloud.AccountCredentials{
		GitHubLogin: "???",
		AccessToken: accessToken,
	}
	if err := cloud.StoreCredentials(creds); err != nil {
		_ = cloud.DeleteStoredCredentials()
		return fmt.Errorf("storing credentials (temporarily)")
	}

	var userResponse apitype.User
	errResp, err := cloud.PulumiRESTCall("GET", "/user", nil, &userResponse)
	if err != nil {
		_ = cloud.DeleteStoredCredentials()
		return fmt.Errorf("error using access token: %v", err)
	}
	if errResp != nil {
		if errResp.Code == 401 {
			return fmt.Errorf("invalid access token")
		}
		return fmt.Errorf("error response from Pulumi API (%d): %s", errResp.Code, errResp.Message)
	}

	// Store the credentials for later.
	creds.GitHubLogin = userResponse.GitHubLogin
	return cloud.StoreCredentials(creds)
}

package azure

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/AlecAivazis/survey.v1/core"

	"github.com/pulumi/pulumi/pkg/workspace"
	survey "gopkg.in/AlecAivazis/survey.v1"
)

var keyRegEx = regexp.MustCompile("^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{4})$")

func isValidAccountKey(key string) bool {
	return keyRegEx.MatchString(key)
}

// Login will handle getting the user's Azure Blob Storage
// credentials and writing them into the current workspace.
func Login(ctx context.Context, cloudURL string) error {
	// If we have a saved access token, and it is valid, use it.
	existingToken, err := workspace.GetAccessToken(cloudURL)
	if err == nil && existingToken != "" {
		if !isValidAccountKey(existingToken) {
			return fmt.Errorf("the format of the existing Azure storage account key is invalid")
		}

		// TODO: Validate existing token works

		return nil
	}

	accessToken := os.Getenv(accessTokenEnvVar)

	if accessToken != "" {
		if !isValidAccountKey(accessToken) {
			return fmt.Errorf("the format of the provided Azure storage account key is invalid")
		}
		fmt.Printf("Using access token from %s\n", accessTokenEnvVar)
	} else {

		// TODO: Support other login modes.
		// This will likely mean moving away from
		// using storage credentials directly and
		// over to using Azure Active Directory.

		var response string

		core.QuestionIcon = ""
		prompt := "\bPlease enter your Azure storage account key:\n"
		if err = survey.AskOne(&survey.Input{
			Message: prompt,
		}, &response, nil); err != nil {
			return fmt.Errorf("no Azure storage account key provided")
		}
		if response == "" {
			return fmt.Errorf("empty Azure storage account key provided")
		}
		if !isValidAccountKey(response) {
			return fmt.Errorf("the format of the provided Azure storage account key is invalid")
		}
		accessToken = response
		// TODO: Validate existing token works
	}

	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return err
	}

	return nil
}

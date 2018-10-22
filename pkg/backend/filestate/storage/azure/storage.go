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

func isValidateCredential(url, accountKey string) bool {
	bucket, err := NewBucket(url, accountKey)
	if err != nil {
		return false
	}
	if _, err = bucket.ListFiles(context.Background(), ""); err != nil {
		return false
	}
	return true
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

		if isValidateCredential(cloudURL, existingToken) {
			return nil // Credential is ok to use
		}
	}

	accessToken := os.Getenv(accessTokenEnvVar)

	if accessToken != "" {
		if !isValidAccountKey(accessToken) {
			return fmt.Errorf("the format of the provided Azure storage account key is invalid")
		}

		fmt.Printf("Using access token from %s\n", accessTokenEnvVar)

		if !isValidateCredential(cloudURL, accessToken) {
			return fmt.Errorf("the provided Azure storage account key and URL are not valid")
		}
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

		if !isValidateCredential(cloudURL, accessToken) {
			return fmt.Errorf("the provided Azure storage account key and URL are not valid")
		}
	}

	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return err
	}

	return nil
}

package azure

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Login will handle getting the user's Azure Blob Storage
// credentials and writing them into the current workspace.
func Login(ctx context.Context, cloudURL string) error {
	// If we have a saved access token, and it is valid, use it.
	existingToken, err := workspace.GetAccessToken(cloudURL)
	if err == nil && existingToken != "" {
		//TODO: Validate existing token works
		return nil
	}

	accessToken := os.Getenv(accessTokenEnvVar)

	if accessToken != "" {
		fmt.Printf("Using access token from %s\n", accessTokenEnvVar)
	} else {
		// TODO: Support other login modes
		return errors.Errorf("Please login to your Azure backend using `pulumi login`")
	}

	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return err
	}

	return nil
}

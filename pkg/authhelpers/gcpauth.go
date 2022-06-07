package authhelpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/oauth2/google"

	"gocloud.dev/blob/gcsblob"

	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudkms/v1"

	"gocloud.dev/blob"
	"gocloud.dev/gcp"
)

type GoogleCredentials struct {
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	ClientID     string `json:"client_id"`
}

// ResolveGoogleCredentials loads the google credentials using the pulumi-specific
// logic first, falling back to the DefaultCredentials resoulution after.
func ResolveGoogleCredentials(ctx context.Context) (*google.Credentials, error) {
	// GOOGLE_CREDENTIALS aren't part of the gcloud standard authorization variables
	// but the GCP terraform provider uses this variable to allow users to authenticate
	// with the contents of a credentials.json file instead of just a file path.
	// https://www.terraform.io/docs/backends/types/gcs.html
	if creds := os.Getenv("GOOGLE_CREDENTIALS"); creds != "" {
		// We try $GOOGLE_CREDENTIALS before gcp.DefaultCredentials
		// so that users can override the default creds
		credentials, err := google.CredentialsFromJSON(ctx, []byte(creds), storage.ScopeReadWrite, cloudkms.CloudkmsScope)
		if err != nil {
			return nil, fmt.Errorf("unable to parse credentials from $GOOGLE_CREDENTIALS: %w", err)
		}
		return credentials, nil
	}

	// PULUMI_GOOGLE_CREDENTIALS_HELPER isn't part of the gcloud standard authorization
	// but it allows the end user to be flexible on how pulumi gets the credentials
	// and guarantees that there's no env name clash with anything else down the stack that
	// can look for the default GCP credentials.
	if credsHelper := os.Getenv("PULUMI_GOOGLE_CREDENTIALS_HELPER"); credsHelper != "" {
		// We try $PULUMI_GOOGLE_CREDENTIALS_HELPER before gcp.DefaultCredentials
		// so that users can override the default creds
		creds, err := exec.Command(credsHelper).Output()
		if err != nil {
			return nil, errors.Wrap(err, "unable to run the $PULUMI_GOOGLE_CREDENTIALS_HELPER")
		}
		credentials, err := google.CredentialsFromJSON(ctx, creds, storage.ScopeReadWrite, cloudkms.CloudkmsScope)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse credentials from $PULUMI_GOOGLE_CREDENTIALS_HELPER")
		}
		return credentials, nil
	}

	// DefaultCredentials will attempt to load creds in the following order:
	// 1. a file located at $GOOGLE_APPLICATION_CREDENTIALS
	// 2. application_default_credentials.json file in ~/.config/gcloud or $APPDATA\gcloud
	credentials, err := gcp.DefaultCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to find gcp credentials: %w", err)
	}
	return credentials, nil
}

func GoogleCredentialsMux(ctx context.Context) (*blob.URLMux, error) {
	credentials, err := ResolveGoogleCredentials(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "missing google credentials")
	}

	client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), credentials.TokenSource)
	if err != nil {
		return nil, err
	}

	options := gcsblob.Options{}
	account := GoogleCredentials{}
	err = json.Unmarshal(credentials.JSON, &account)
	if err == nil && account.ClientEmail != "" && account.PrivateKey != "" {
		options.GoogleAccessID = account.ClientEmail
		options.PrivateKey = []byte(account.PrivateKey)
	}

	blobmux := &blob.URLMux{}
	blobmux.RegisterBucket(gcsblob.Scheme, &gcsblob.URLOpener{
		Client:  client,
		Options: options,
	})

	return blobmux, nil
}

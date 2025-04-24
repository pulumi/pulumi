// Copyright 2019-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"gocloud.dev/blob/gcsblob"

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
func ResolveGoogleCredentials(ctx context.Context, scope string) (*google.Credentials, error) {
	// GOOGLE_CREDENTIALS aren't part of the gcloud standard authorization variables
	// but the GCP terraform provider uses this variable to allow users to authenticate
	// with the contents of a credentials.json file instead of just a file path.
	// https://www.terraform.io/docs/backends/types/gcs.html
	if creds := os.Getenv("GOOGLE_CREDENTIALS"); creds != "" {
		// We try $GOOGLE_CREDENTIALS before gcp.DefaultCredentials
		// so that users can override the default creds
		credentials, err := google.CredentialsFromJSON(ctx, []byte(creds), scope)
		if err != nil {
			return nil, fmt.Errorf("unable to parse credentials from $GOOGLE_CREDENTIALS: %w", err)
		}
		return credentials, nil
	}

	// GOOGLE_OAUTH_ACCESS_TOKEN isnt't part of the gcloud standard authorization variables
	// but the GCP terraform provider uses this variable to allow users to authenticate
	// with a temporary access token obtained from the Google Authorization Server instead
	// of just a file path or credentials.json.
	// https://www.terraform.io/docs/backends/types/gcs.html
	if creds := os.Getenv("GOOGLE_OAUTH_ACCESS_TOKEN"); creds != "" {
		// We try $GOOGLE_OAUTH_ACCESS_TOKEN before gcp.DefaultCredentials
		// so that users can override the default creds

		return &google.Credentials{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: creds}),
		}, nil
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
	credentials, err := ResolveGoogleCredentials(ctx, storage.ScopeReadWrite)
	if err != nil {
		return nil, fmt.Errorf("missing google credentials: %w", err)
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

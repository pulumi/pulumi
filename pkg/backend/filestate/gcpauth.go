package filestate

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"gocloud.dev/blob"
	"gocloud.dev/blob/gcsblob"
	"gocloud.dev/gcp"
	"golang.org/x/oauth2/jwt"
)

type GoogleCredentials struct {
	PrivateKeyId string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	ClientId     string `json:"client_id"`
}

func GoogleCredentialsMux(credentialsJSON string) (*blob.URLMux, error) {
	credentials := GoogleCredentials{}
	err := json.Unmarshal([]byte(credentialsJSON), &credentials)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse $GOOGLE_CREDENTIALS")
	}

	conf := jwt.Config{
		Email:      credentials.ClientEmail,
		PrivateKey: []byte(credentials.PrivateKey),
		Scopes:     []string{storage.ScopeReadWrite},
		TokenURL:   "https://accounts.google.com/o/oauth2/token",
	}
	client := &gcp.HTTPClient{
		Client: *conf.Client(context.TODO()),
	}

	options := gcsblob.Options{
		GoogleAccessID: credentials.ClientEmail,
		PrivateKey:     []byte(credentials.PrivateKey),
	}

	blobmux := &blob.URLMux{}
	blobmux.RegisterBucket(gcsblob.Scheme, &gcsblob.URLOpener{
		Client:  client,
		Options: options,
	})

	return blobmux, nil
}

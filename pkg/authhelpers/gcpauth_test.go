package authhelpers

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest
func TestResolveGoogleCredentials_ValidCredentials(t *testing.T) {
	t.Setenv("GOOGLE_CREDENTIALS", `{
		"type": "service_account",
		"project_id": "your-project-id",
		"private_key_id": "your-private-key-id",
		"private_key": "your-private-key",
		"client_email": "your-client-email",
		"client_id": "your-client-id"
	}`)

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.NoError(t, err)
	assert.NotNil(t, credentials)

	var creds map[string]interface{}
	err = json.Unmarshal([]byte(os.Getenv("GOOGLE_CREDENTIALS")), &creds)
	assert.NoError(t, err)
	assert.Equal(t, creds["type"], "service_account")
	assert.Equal(t, creds["project_id"], "your-project-id")
	assert.Equal(t, creds["private_key_id"], "your-private-key-id")
	assert.Equal(t, creds["private_key"], "your-private-key")
	assert.Equal(t, creds["client_email"], "your-client-email")
	assert.Equal(t, creds["client_id"], "your-client-id")
}

//nolint:paralleltest
func TestResolveGoogleCredentials_InvalidCredentials(t *testing.T) {
	t.Setenv("GOOGLE_CREDENTIALS", `{}`)

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.Error(t, err, "Expected an error")
	assert.Nil(t, credentials, "Expected nil credentials")
}

//nolint:paralleltest
func TestResolveGoogleCredentials_OAuthAccessToken(t *testing.T) {
	expectedAccessToken := "your-access-token"
	t.Setenv("GOOGLE_OAUTH_ACCESS_TOKEN", expectedAccessToken)

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.NoError(t, err)
	assert.NotNil(t, credentials)

	token, err := credentials.TokenSource.Token()
	assert.NoError(t, err)

	actualAccessToken := token.AccessToken
	assert.Equal(t, expectedAccessToken, actualAccessToken)
}

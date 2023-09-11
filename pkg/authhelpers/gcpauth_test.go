package authhelpers

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGoogleCredentials_ValidCredentials(t *testing.T) {
	t.Parallel()
	os.Setenv("GOOGLE_CREDENTIALS", `{
		"type": "service_account",
		"project_id": "your-project-id",
		"private_key_id": "your-private-key-id",
		"private_key": "your-private-key",
		"client_email": "your-client-email",
		"client_id": "your-client-id"
	}`)

	defer os.Unsetenv("GOOGLE_CREDENTIALS")

	os.Setenv("GOOGLE_CREDENTIALS", os.Getenv("GOOGLE_CREDENTIALS"))

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.NoError(t, err)
	assert.NotNil(t, credentials)

	var creds map[string]interface{}
	err = json.Unmarshal([]byte(os.Getenv("GOOGLE_CREDENTIALS")), &creds)
	assert.NoError(t, err)
}

func TestResolveGoogleCredentials_InvalidCredentials(t *testing.T) {
	t.Parallel()
	os.Setenv("GOOGLE_CREDENTIALS", `{}`)

	defer os.Unsetenv("GOOGLE_CREDENTIALS")

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.Error(t, err, "Expected an error")
	assert.Nil(t, credentials, "Expected nil credentials")
}

func TestResolveGoogleCredentials_OAuthAccessToken(t *testing.T) {
	t.Parallel()
	os.Setenv("GOOGLE_OAUTH_ACCESS_TOKEN", "your-access-token")
	defer os.Unsetenv("GOOGLE_OAUTH_ACCESS_TOKEN")

	ctx := context.Background()
	scope := "some-scope"

	credentials, err := ResolveGoogleCredentials(ctx, scope)

	assert.NoError(t, err)
	assert.NotNil(t, credentials)

	token, err := credentials.TokenSource.Token()
	assert.NoError(t, err)
	assert.Empty(t, os.Getenv("GOOGLE_CREDENTIALS"))
	assert.Equal(t, "your-access-token", token.AccessToken)
	assert.Equal(t, "your-access-token", os.Getenv("GOOGLE_OAUTH_ACCESS_TOKEN"))
}

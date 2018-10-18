package local

import (
	"context"

	"github.com/pulumi/pulumi/pkg/workspace"
)

// Login will write a phony entry into the user's workspace
// to identify the local file provider
func Login(ctx context.Context, url string) error {
	return workspace.StoreAccessToken(url, "", true)
}

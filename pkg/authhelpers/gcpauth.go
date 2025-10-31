package authhelpers

import authhelpers "github.com/pulumi/pulumi/sdk/v3/pkg/authhelpers"

type GoogleCredentials = authhelpers.GoogleCredentials

// ResolveGoogleCredentials loads the google credentials using the pulumi-specific
// logic first, falling back to the DefaultCredentials resoulution after.
func ResolveGoogleCredentials(ctx context.Context, scope string) (*google.Credentials, error) {
	return authhelpers.ResolveGoogleCredentials(ctx, scope)
}

func GoogleCredentialsMux(ctx context.Context) (*blob.URLMux, error) {
	return authhelpers.GoogleCredentialsMux(ctx)
}


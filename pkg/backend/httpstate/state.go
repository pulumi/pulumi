package httpstate

import httpstate "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate"

func RenewLeaseFunc(client *client.Client, update client.UpdateIdentifier, assumedExpires func() time.Time) func(context.Context, time.Duration, string) (string, time.Time, error) {
	return httpstate.RenewLeaseFunc(client, update, assumedExpires)
}


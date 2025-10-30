package client

import client "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate/client"

// StackIdentifier is the set of data needed to identify a Pulumi Cloud stack.
type StackIdentifier = client.StackIdentifier

// UpdateIdentifier is the set of data needed to identify an update to a Pulumi Cloud stack.
type UpdateIdentifier = client.UpdateIdentifier

// UpdateTokenSource allows the API client to request tokens for an in-progress update as near as possible to the
// actual API call (e.g. after marshaling, etc.).
type UpdateTokenSource = client.UpdateTokenSource

func UserAgent() string {
	return client.UserAgent()
}


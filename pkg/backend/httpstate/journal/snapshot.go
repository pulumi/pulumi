package journal

import journal "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate/journal"

func NewJournaler(ctx context.Context, client *client.Client, update client.UpdateIdentifier, tokenSource tokenSourceCapability, sm secrets.Manager) engine.Journal {
	return journal.NewJournaler(ctx, client, update, tokenSource, sm)
}


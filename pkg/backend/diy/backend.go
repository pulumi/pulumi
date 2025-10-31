package diy

import diy "github.com/pulumi/pulumi/sdk/v3/pkg/backend/diy"

// UpgradeOptions customizes the behavior of the upgrade operation.
type UpgradeOptions = diy.UpgradeOptions

// Backend extends the base backend interface with specific information about diy backends.
type Backend = diy.Backend

const FilePathPrefix = diy.FilePathPrefix

func IsDIYBackendURL(urlstr string) bool {
	return diy.IsDIYBackendURL(urlstr)
}

// New constructs a new diy backend,
// using the given URL as the root for storage.
// The URL must use one of the schemes supported by the go-cloud blob package.
// Thes inclue: file, s3, gs, azblob.
func New(ctx context.Context, d diag.Sink, originalURL string, project *workspace.Project) (Backend, error) {
	return diy.New(ctx, d, originalURL, project)
}

func Login(ctx context.Context, d diag.Sink, url string, project *workspace.Project) (Backend, error) {
	return diy.Login(ctx, d, url, project)
}

// GetLogsForTarget fetches stack logs using the config, decrypter, and checkpoint in the given target.
func GetLogsForTarget(target *deploy.Target, query operations.LogQuery) ([]operations.LogEntry, error) {
	return diy.GetLogsForTarget(target, query)
}


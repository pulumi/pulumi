package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// readProject attempts to detect and read a Pulumi project for the current workspace. If the
// project is successfully detected and read, it is returned along with the path to its containing
// directory, which will be used as the root of the project's Pulumi program.
func ReadProject() (*workspace.Project, string, error) {
	proj, path, err := ReadProjectWithPath()
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
}

// readProjectWithPath attempts to detect and read a Pulumi project for the current workspace. If
// the project is successfully detected and read, it is returned along with the path to the project
// file, which will be used as the root of the project's Pulumi program.
//
// If a project is not found while searching and no other error occurs, workspace.ErrProjectNotFound
// is returned.
func ReadProjectWithPath() (*workspace.Project, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectProjectPathFrom(pwd)
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.LoadProject(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Pulumi project located at %q: %w", path, err)
	}

	return proj, path, nil
}

// BackendInstance is used to inject a backend mock from tests.
var BackendInstance backend.Backend

func IsFilestateBackend(opts display.Options) (bool, error) {
	if BackendInstance != nil {
		return false, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return filestate.IsFileStateBackendURL(url), nil
}

func NonInteractiveCurrentBackend(ctx context.Context) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(cmdutil.Diag(), url)
	}
	return httpstate.NewLoginManager().Current(ctx, cmdutil.Diag(), url)
}

func CurrentBackend(ctx context.Context, opts display.Options) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(cmdutil.Diag(), url)
	}
	return httpstate.NewLoginManager().Login(ctx, cmdutil.Diag(), url, opts)
}

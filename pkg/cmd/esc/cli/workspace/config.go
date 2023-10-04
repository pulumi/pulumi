// Copyright 2023, Pulumi Corporation. All rights reserved.

package workspace

import (
	"errors"
	"io/fs"
)

func (w *Workspace) SetBackendConfigDefaultOrg(backendURL, defaultOrg string) error {
	return w.pulumi.SetBackendConfigDefaultOrg(backendURL, defaultOrg)
}

func (w *Workspace) GetBackendConfigDefaultOrg(backendURL, username string) (string, error) {
	config, err := w.pulumi.GetPulumiConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if cfg, ok := config.BackendConfig[backendURL]; ok && cfg.DefaultOrg != "" {
		return cfg.DefaultOrg, nil
	}
	return username, nil
}

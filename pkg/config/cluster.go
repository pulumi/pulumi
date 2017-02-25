// Copyright 2016 Pulumi, Inc. All rights reserved.

package config

// Cluster describes a predefined cloud runtime target, including its OS and Scheduler combination.
type Cluster struct {
	Name        string            `json:"name"`                  // the friendly name of the cluster entry.
	Default     bool              `json:"default,omitempty"`     // a single target can carry default settings.
	Description string            `json:"description,omitempty"` // a human-friendly description of this target.
	Cloud       string            `json:"cloud,omitempty"`       // the cloud target.
	Scheduler   string            `json:"scheduler,omitempty"`   // the cloud scheduler target.
	Settings    map[string]string `json:"settings,omitempty"`    // any options passed to the cloud provider.
}

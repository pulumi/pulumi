// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apitype

// Organization represents a Pulumi organization.
type Organization struct {
	GitHubLogin string `json:"githubLogin"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatarUrl"`

	Clouds       []Cloud `json:"clouds"`
	DefaultCloud string  `json:"defaultCloud"`

	Repositories []PulumiRepository `json:"repos"`
}

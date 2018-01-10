// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apitype

// User represents a Pulumi user.
type User struct {
	ID            string         `json:"id"`
	GitHubLogin   string         `json:"githubLogin"`
	Name          string         `json:"name"`
	AvatarURL     string         `json:"avatarUrl"`
	Organizations []Organization `json:"organizations"` // TODO: This used to be interface{} in pulumi
}

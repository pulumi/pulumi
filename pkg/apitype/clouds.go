package apitype

// Cloud describes a Pulumi Private Cloud (PPC).
type Cloud struct {
	Name              string `json:"name"`
	OrganizationLogin string `json:"organizationLogin"`
	Endpoint          string `json:"endpoint"`

	// IsDefault flags the Cloud as being the default cloud for new stacks in
	// the parent organization.
	IsDefault bool `json:"isDefault"`

	StackLimit int `json:"stackLimit"`
}

// CreateCloudRequest is the request to associate a new Cloud with an organization.
// (The target organization is inferred from the REST URL.)
type CreateCloudRequest struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	AccessToken string `json:"accessToken"`
}

// CloudConfigurationRule is a rule for how the Cloud manages Pulumi program configuration.
type CloudConfigurationRule struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SetCloudConfigurationRequest is the request to set the cloud's configuration.
// It is expected to be a full replacement, not a partial update.
type SetCloudConfigurationRequest struct {
	Configuration []CloudConfigurationRule `json:"configuration"`
}

// CloudStatus describes the state of a Pulumi Private Cloud.
type CloudStatus struct {
	Status   string            `json:"status"`
	Versions map[string]string `json:"versions"`
}

package engine

type PulumiMapperEvent struct {
	AccountId        string `json:"accountId"`
	IntegrationId    string `json:"integrationId"`
	ProjectName      string `json:"projectName"`
	StackName        string `json:"stackName"`
	OrganizationName string `json:"organizationName"`
	LastUpdated      int64  `json:"lastUpdated"`
	ResourceCount    int    `json:"resourceCounts"`
}

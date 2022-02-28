package engine

type PulumiNode struct {
	Type                string
	AccountId           string
	StackId             string
	Iac                 string
	PulumiIntegrationId string
	Arn                 string
	Region              string
	ProviderAccountId   string
	AwsIntegration      string
	K8sIntegration      string
	Location            string
	Name                string
	ResourceId          string
	AssetId             string
	Kind                string
	IsOrchestrator      bool
	UpdatedAt           int64
	Metadata            PulumiIacMetadata
	ObjectType          string
	Attributes          string
}

type PulumiIacMetadata struct {
	StackId          string                   `json:"stackId"`
	StackName        string                   `json:"stackName"`
	ProjectName      string                   `json:"projectName"`
	OrganizationName string                   `json:"organizationName"`
	PulumiType       string                   `json:"pulumiType"`
	PulumiState      string                   `json:"pulumiState"`
	PulumiDrifts     []map[string]interface{} `json:"pulumiDrifts"`
}

package apitype

// GitHubRepo is a subset of the information returned from the GitHub Repo API.
type GitHubRepo struct {
	OwnerLogin  string `json:"ownerLogin"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"isPrivate"`

	HTMLURL  string   `json:"htmlUrl"`
	Homepage string   `json:"homepage"`
	Topics   []string `json:"topics"`
}

// PulumiRepository is a grouping of "Projects". We also return a subset of the organization's
// GitHub repo with the same name, should it exist.
type PulumiRepository struct {
	OrgName string `json:"orgName"`
	Name    string `json:"name"`

	GitHubRepo *GitHubRepo     `json:"githubRepo"`
	Projects   []PulumiProject `json:"projects"`
}

// PulumiProject has a 1:1 correspondence to pulumi.yaml files.
type PulumiProject struct {
	OrgName  string   `json:"orgName"`
	RepoName string   `json:"repoName"`
	Name     string   `json:"name"`
	Stacks   []string `json:"stacks"`
}

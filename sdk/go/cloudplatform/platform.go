package cloudplatform

type AutomationJobArgs struct {
	Repo           string
	Dir            string
	PreRunCommands []string
	Entrypoint     string
	Environemnt    []string
	Mode           string
	Schedule       string
}

type Platform interface {
	CreateAutomationJob(name string, args AutomationJobArgs)
	Search() ([]Resource, error)
}

type CloudPlatform struct{}

func NewCloudPlatform(org string) (Platform, error) {

	return &CloudPlatform{}, nil
}

func (c *CloudPlatform) CreateAutomationJob(name string, args AutomationJobArgs) {
	// TODO
}

func (c *CloudPlatform) Search() ([]Resource, error) {
	// hit the pulumi cloud resource search API
	return nil, nil
}

type Stack interface {
}

type CloudStack struct{}

func NewCloudStack() (Stack, error) {
	return &CloudStack{}, nil
}

type Resource interface{}

type CloudResource struct{}

func NewCloudResource() (Resource, error) {
	return &CloudResource{}, nil
}

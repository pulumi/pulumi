package cloudplatform

func ExampleAutomationJob() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	p.CreateAutomationJob("multi-region-deployer", AutomationJobArgs{
		Repo: "github.com/pulumi/automation-api-examples",
		Dir:  "nodejs/multi-region-deployer",
		PreRunCommands: []string{
			"npm install",
		},
		Entrypoint: "npm start",
		// automaticaly provide credentials for stripe and AWS in the dev environment
		Environemnt: []string{
			"aws-dev",
			"stripe-dev",
		},
		Mode: "cron",
		// deploy every day at 8am
		Schedule: "0 8 * * *",
	})
}

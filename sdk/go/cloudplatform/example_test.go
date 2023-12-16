package cloudplatform

import (
	"fmt"
	"time"
)

func ExampleAutomationJob() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	// run any automation API program in Pulumi Cloud
	p.CreateAutomationJob("multi-region-deployer", AutomationJobArgs{
		Repo: "github.com/pulumi/automation-api-examples",
		Dir:  "nodejs/multi-region-deployer",
		PreRunCommands: []string{
			"npm install",
		},
		Entrypoint: "npm start",
		// automaticaly provide credentials for stripe and AWS via ESC
		Environemnt: []string{
			"aws-dev",
			"stripe-dev",
		},
		Mode: "cron",
		// run every day at 8am
		Schedule: "0 8 * * *",
	})
}

func ExampleAutomationJobYAML() {
	// TODO is there a verison of automation jobs that accepts YAML?
	// Can you create an automation job, and then expose some sort of schema or template
	// to customize an instance of it?
	/*
		p.RegisterAutomationJob("drift-checker", AutomationJobArgs{
			Repo: "github.com/pulumi/automation-api-examples",
			Dir:  "nodejs/drift-checker",
			PreRunCommands: []string{
				"npm install",
			},
			Entrypoint: "npm start",
			// automaticaly provide credentials for stripe and AWS via ESC
			Environemnt: []string{
				"aws-dev",
				"stripe-dev",
			},
			Mode: "cron",
			// run every day at 8am
			Schedule: "inputs.schedule",
			Schema: {
				schedule: "string",
				remediate: "boolean",
				notificationChannel: "string",
				notificationMode" "string",
			}
		})

		p.NewAutomationJob(`
			name: my-drift-checker
			kind: drift-checker
			inputs:
				schedule: "0 8 * * *"
				remediate: true
				notificationMode: "slack"
				notificationChannel: "#drift-alerts"
		`)

	*/
}

func ExampleCostManagement() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	// find all G5 GPU EC2 instances
	res, err := p.Search(`type:"aws:ec2/instance:instance" .instanceType:G5`)
	if err != nil {
		panic(err)
	}

	// create a list of all matching resource IDs grouped by stack
	stacksToIds := map[Stack][]string{}
	for _, resource := range res {
		if stacksToIds[resource.Stack()] == nil {
			stacksToIds[resource.Stack()] = []string{resource.ID()}
		} else {
			stacksToIds[resource.Stack()] = append(stacksToIds[resource.Stack()], resource.ID())
		}
	}

	// iterate through all stacks in the results set
	for stack, ids := range stacksToIds {
		// check to ensure deployments is configured
		if stack.SupportsDeployments() {
			// run a destroy targeting only the GPUs
			stack.RunDeployment(DeploymentArgs{
				Targets:   ids,
				Operation: "destroy",
			})
		}
	}
}

func ExampleSearchPlusPolicyViolations() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	// find all lambdas running nodejs v12 (deprecated and in need of security upgrade)
	res, err := p.Search(`.runtime:nodejs12`)
	if err != nil {
		panic(err)
	}

	// for each resource report a violation to Pulumi Cloud that will show up on our dashboard
	// along with check to re-validate the resource upon next update
	for _, resource := range res {
		securityLevel := "HIGH"
		note := "NodeJS LTS now at v20, resource at v12 and needs security updates"
		revalidationQuery := fmt.Sprintf(".runtime:nodejs12 ID:%s", resource.ID())
		resource.ReportPolicyViolation(securityLevel, note, revalidationQuery)
	}
}

func ExampleServiceCreationFromTemplate() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	t := p.GetServiceTemplate("kubernetes-cluster")
	t.TemplateAndDeploy(TemplateArgs{
		DestinationRepo: "github.com/acme/new-kubernetes-app",
		Dir:             "infra",
		PushToDeploy:    true,
		PRPreview:       true,
		ReviewStacks:    true,
		Drift:           true,
		TTLSeconds:      42,
		Environment: []string{
			"aws-dev",
		},
	})
}

func ExampleDriftDetectionImplementation() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	stacks := p.ListStacks(ListStackArgs{})
	for _, stack := range stacks {
		if stack.SupportsDeployments() || stack.HasEnvironment() {
			stack.RunDeployment(DeploymentArgs{
				// we don't need source code for refresh/destroy
				AcquireSource:   false,
				Operation:       "refresh",
				ExpectNoChanges: true,
				OnFailure: &NotificationArgs{
					Type:    "slack", // could be "email" or "policy-violation"
					Route:   "#drift-alerts",
					Message: fmt.Sprintf("drift check failed for stack %s", stack.Name()),
				},
			})
		}
	}
}

func ExampleDriftJob() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	// run any automation API program in Pulumi Cloud
	p.CreateAutomationJob("drift-checker", AutomationJobArgs{
		Repo:       "github.com/pulumi/automation-api-examples",
		Dir:        "go/drift",
		Entrypoint: "go run main.go",
		Mode:       "cron",
		// run every day at 8am
		Schedule: "0 8 * * *",
	})
}

func ExampleTTL() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	stacks := p.ListStacks(ListStackArgs{
		tags: []string{
			"key eq TTL",
			fmt.Sprintf("value lt %s", time.Now().UTC()),
		},
	})
	for _, stack := range stacks {
		if stack.SupportsDeployments() || stack.HasEnvironment() {
			stack.RunDeployment(DeploymentArgs{
				// we don't need source code for refresh/destroy
				AcquireSource: false,
				Operation:     "destroy",
				// run pulumi stack rm as final step
				DeleteStack: true,
				OnFailure: &NotificationArgs{
					Type:    "email",
					Route:   "platform-ops@acme.com",
					Message: fmt.Sprintf("TTL destroy failed for stack %s", stack.Name()),
				},
			})
		}
	}
}

func ExampleTTLJob() {
	org := "acme"
	p, err := NewCloudPlatform(org)
	if err != nil {
		panic(err)
	}

	p.CreateAutomationJob("ttl-reaper", AutomationJobArgs{
		Repo:       "github.com/pulumi/automation-api-examples",
		Dir:        "go/ttl",
		Entrypoint: "go run main.go",
		Mode:       "cron",
		// run hourly
		Schedule: "0 * * * *",
	})
}

func ExampleSynkScanning() {

}

func ExampleWorkflow() {

}

func ExampleSMSApproval() {

}

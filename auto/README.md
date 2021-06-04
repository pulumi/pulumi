# Automation API

Programmatic infrastructure.

## Godocs
See the full godocs for the most extensive and up to date information including full examples coverage: 

https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/auto?tab=doc

## Examples

Multiple full working examples with detailed walkthroughs can be found in this repo:

https://github.com/pulumi/automation-api-examples


## Overview

Package auto contains the Pulumi Automation API, the programmatic interface for driving Pulumi programs
without the CLI.
Generally this can be thought of as encapsulating the functionality of the CLI (`pulumi up`, `pulumi preview`,
`pulumi destroy`, `pulumi stack init`, etc.) but with more flexibility. This still requires a
CLI binary to be installed and available on your $PATH.

In addition to fine-grained building blocks, Automation API provides three out of the box ways to work with Stacks:

1. Programs locally available on-disk and addressed via a filepath (NewStackLocalSource)
```go
    stack, err := NewStackLocalSource(ctx, "myOrg/myProj/myStack", filepath.Join("..", "path", "to", "project"))
```
2. Programs fetched from a Git URL (NewStackRemoteSource)
```go
	stack, err := NewStackRemoteSource(ctx, "myOrg/myProj/myStack", GitRepo{
		URL:         "https:github.com/pulumi/test-repo.git",
		ProjectPath: filepath.Join("project", "path", "repo", "root", "relative"),
    })
```
3. Programs defined as a function alongside your Automation API code (NewStackInlineSource)
```go
	 stack, err := NewStackInlineSource(ctx, "myOrg/myProj/myStack", "myProj", func(pCtx *pulumi.Context) error {
		bucket, err := s3.NewBucket(pCtx, "bucket", nil)
		if err != nil {
			return err
		}
		pCtx.Export("bucketName", bucket.Bucket)
		return nil
     })
```

Each of these creates a stack with access to the full range of Pulumi lifecycle methods
(up/preview/refresh/destroy), as well as methods for managing config, stack, and project settings.

```go
	 err := stack.SetConfig(ctx, "key", ConfigValue{ Value: "value", Secret: true })
	 preRes, err := stack.Preview(ctx)
	 detailed info about results
     fmt.Println(preRes.prev.Steps[0].URN)
```

The Automation API provides a natural way to orchestrate multiple stacks,
feeding the output of one stack as an input to the next as shown in the package-level example below.
The package can be used for a number of use cases:

- Driving pulumi deployments within CI/CD workflows
- Integration testing
- Multi-stage deployments such as blue-green deployment patterns
- Deployments involving application code like database migrations
- Building higher level tools, custom CLIs over pulumi, etc
- Using pulumi behind a REST or GRPC API
- Debugging Pulumi programs (by using a single main entrypoint with "inline" programs)

To enable a broad range of runtime customization the API defines a `Workspace` interface.
A Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
Workspaces are used to manage the execution environment, providing various utilities such as plugin
installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
Every Stack including those in the above examples are backed by a Workspace which can be accessed via:
```go
	 w = stack.Workspace()
     err := w.InstallPlugin("aws", "v3.2.0")
```
Workspaces can be explicitly created and customized beyond the three Stack creation helpers noted above:
```go
	 w, err := NewLocalWorkspace(ctx, WorkDir(filepath.Join(".", "project", "path"), PulumiHome("~/.pulumi"))
     s := NewStack(ctx, "org/proj/stack", w)
```
A default implementation of workspace is provided as `LocalWorkspace`. This implementation relies on Pulumi.yaml
and Pulumi.<stack>.yaml as the intermediate format for Project and Stack settings. Modifying ProjectSettings will
alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.<stack>.yaml file.
This is identical to the behavior of Pulumi CLI driven workspaces. Custom Workspace
implementations can be used to store Project and Stack settings as well as Config in a different format,
such as an in-memory data structure, a shared persistent SQL database, or cloud object storage. Regardless of
the backing Workspace implementation, the Pulumi SaaS Console will still be able to display configuration
applied to updates as it does with the local version of the Workspace today.

The Automation API also provides error handling utilities to detect common cases such as concurrent update
conflicts:

```go
	uRes, err :=stack.Up(ctx)
	if err != nil && IsConcurrentUpdateError(err) { /* retry logic here */ }
```

## Developing the Godocs
This repo has extensive examples and godoc content. To test out your changes locally you can do the following:

1. enlist in the appropriate pulumi branch:
2. cd $GOPATH/src/github.com/pulumi/pulumi/sdk/go/auto
3. godoc -http=:6060
4. Navigate to http://localhost:6060/pkg/github.com/pulumi/pulumi/sdk/v3/go/auto/

## Known Issues

Please upvote issues, add comments, and open new ones to help prioritize our efforts:
https://github.com/pulumi/pulumi/issues?q=is%3Aissue+is%3Aopen+label%3Aarea%2Fautomation-api

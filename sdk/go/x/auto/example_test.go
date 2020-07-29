package auto

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
)

func Example() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// define config to be used in the stack
	awsStackConfig := &StackOverrides{
		Config: map[string]string{"aws:region": "us-west-2"},
	}
	bucketStack := StackSpec{
		Name:      "dev_bucket",
		Project:   bucketProj,
		Overrides: awsStackConfig,
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	// -- pulumi up --
	bucketRes, err := s.Up()
	if err != nil {
		return err
	}

	// This stack puts an object in the bucket created in the previous stack
	objProj := ProjectSpec{
		Name: "object_provider",
		InlineSource: func(ctx *pulumi.Context) error {
			obj, err := s3.NewBucketObject(ctx, "object", &s3.BucketObjectArgs{
				Bucket:  pulumi.String(bucketRes.Outputs["bucketName"].(string)),
				Content: pulumi.String("Hello World!"),
			})
			if err != nil {
				return err
			}
			ctx.Export("objKey", obj.Key)
			return nil
		},
	}

	objStack := StackSpec{
		Name:      "dev_obj",
		Project:   objProj,
		Overrides: awsStackConfig,
	}

	// initialize stack
	os, err := NewStack(objStack)
	if err != nil {
		return err
	}

	// -- pulumi up --
	objRes, err := os.Up()
	if err != nil {
		return err
	}
	// Success!
	fmt.Println(objRes.Summary.Result)
	return nil
}

func ExampleNewStack() error {
	// a project is the container for a particular Pulumi program
	ps := ProjectSpec{
		Name: "proj",
		// path to our local on-disk Pulumi program directory
		SourcePath: filepath.Join("..", "proj"),
	}
	// a stack is an instance of a Pulumi program with it's own configuration
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	// NewStack selects a stack if one already exists, and otherwise creates a new one
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleNewStack_sourceRoot() error {
	ps := ProjectSpec{
		Name: "proj",
		// path to our local on-disk Pulumi project directory
		SourcePath: filepath.Join("..", "proj"),
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleNewStack_remote() error {
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		// a Pulumi program hosted in a git repo
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// specify config expected by the Pulumi program
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleNewStack_remoteWithSetup() error {
	// the name of our compiled pulumi Program
	binName := "exampleBinary"
	// a setup function that will run after enlistment in the remote repo
	// builds the `exampleBinary` executable
	setupFn := func(path string) error {
		cmd := exec.Command("go", "build", "-o", binName, "main.go")
		cmd.Dir = path
		return cmd.Run()
	}
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
			Setup:       setupFn,
		},
		Overrides: &ProjectOverrides{
			// customizations to our pulumi.yaml
			Project: &workspace.Project{
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
					// use a pre-built binary rather than invoking the program via `go run ...`
					"binary": binName,
				}),
			},
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// config required by the program
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleNewStack_inlineSource() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		// this program creates an AWS S3 Bucket
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// define config to be used in the stack
	awsStackConfig := &StackOverrides{
		Config: map[string]string{"aws:region": "us-west-2"},
	}
	bucketStack := StackSpec{
		Name:      "dev_bucket",
		Project:   bucketProj,
		Overrides: awsStackConfig,
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	return nil
}

func ExampleNewStack_configAndSecrets() error {
	ps := ProjectSpec{
		Name: "proj",
		// this inline pulumi Program just exports three values
		// 1. a static value
		// 2. a value read from config key=bar
		// 3. a secret read from config key=buzz
		InlineSource: func(ctx *pulumi.Context) error {
			c := config.New(ctx, "")
			ctx.Export("exp_static", pulumi.String("foo"))
			ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
			ctx.Export("exp_secret", c.GetSecret("buzz"))
			return nil
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// set the config required by the program
			Config: map[string]string{"bar": "abc"},
			// set the secrets required by the program, will be stored encrypted
			Secrets: map[string]string{"buzz": "secret"},
		},
	}

	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleSetupFn() error {
	// the name of our compiled pulumi Program
	binName := "exampleBinary"
	// a setup function that will run after enlistment in the remote repo
	// builds the `exampleBinary` executable
	setupFn := func(path string) error {
		cmd := exec.Command("go", "build", "-o", binName, "main.go")
		cmd.Dir = path
		return cmd.Run()
	}
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
			Setup:       setupFn,
		},
		Overrides: &ProjectOverrides{
			// customizations to our pulumi.yaml
			Project: &workspace.Project{
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
					// use a pre-built binary rather than invoking the program via `go run ...`
					"binary": binName,
				}),
			},
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// config required by the program
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleIsCompilationError() error {
	ps := ProjectSpec{
		Name:       "compilation_error",
		SourcePath: filepath.Join("..", "project_with_compilation_error"),
	}

	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	_, err = s.Up()
	if err != nil {
		if IsCompilationError(err) {
			return errors.Wrap(err, "program failed to build")
		}
		return err
	}

	return nil
}

func ExampleIsRuntimeError() error {
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join("..", "project_with_runtime_error"),
	}

	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	_, err = s.Up()
	if err != nil {
		if IsRuntimeError(err) {
			return errors.Wrap(err, "program encountered runtime error")
		}
		return err
	}

	return nil
}

func ExampleIsConcurrentUpdateError() error {
	ps := ProjectSpec{
		Name:       "concurrent_error",
		SourcePath: filepath.Join("..", "project_with_update_in_progress"),
	}

	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	_, err = s.Up()
	if err != nil {
		if IsConcurrentUpdateError(err) {
			return errors.Wrap(err, "another update is already in progress")
		}
		return err
	}

	return nil
}

func ExampleIsUnexpectedEngineError() error {
	ps := ProjectSpec{
		Name:       "engine_error",
		SourcePath: filepath.Join("..", "project_with_engine_error"),
	}

	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	_, err = s.Up()
	if err != nil {
		if IsUnexpectedEngineError(err) {
			return errors.Wrap(err, "pulumi engine encountered an error")
		}
		return err
	}

	return nil
}

func ExampleProjectOverrides() error {
	// the name of our compiled pulumi Program
	binName := "exampleBinary"
	// a setup function that will run after enlistment in the remote repo
	// builds the `exampleBinary` executable
	setupFn := func(path string) error {
		cmd := exec.Command("go", "build", "-o", binName, "main.go")
		cmd.Dir = path
		return cmd.Run()
	}
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
			Setup:       setupFn,
		},
		Overrides: &ProjectOverrides{
			// customizations to our pulumi.yaml
			Project: &workspace.Project{
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
					// use a pre-built binary rather than invoking the program via `go run ...`
					"binary": binName,
				}),
			},
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// config required by the program
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleProjectSpec_inlineSource() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		// this program creates an AWS S3 Bucket
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// define config to be used in the stack
	awsStackConfig := &StackOverrides{
		Config: map[string]string{"aws:region": "us-west-2"},
	}
	bucketStack := StackSpec{
		Name:      "dev_bucket",
		Project:   bucketProj,
		Overrides: awsStackConfig,
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	return nil
}

func ExampleProjectSpec_sourceRoot() error {
	ps := ProjectSpec{
		Name: "proj",
		// path to our local on-disk Pulumi project directory
		SourcePath: filepath.Join("..", "proj"),
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
	}

	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleProjectSpec_remote() error {
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		// a Pulumi program hosted in a git repo
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
		},
	}
	ss := StackSpec{
		Name:    "devStack",
		Project: ps,
		Overrides: &StackOverrides{
			// specify config expected by the Pulumi program
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	return nil
}

func ExampleStackOverrides() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		// this program creates an AWS S3 Bucket
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// a stack spec descripes an instance of a Pulumi program
	bucketStack := StackSpec{
		Name:    "dev_bucket",
		Project: bucketProj,
		// overrides corresponding to pulumi.<stack>.yaml
		Overrides: &StackOverrides{
			// define config to be used in the stack
			Config: map[string]string{"aws:region": "us-west-2"},
			ProjectStack: &workspace.ProjectStack{
				// specify a user managed key
				EncryptedKey: "bring your own encryption key",
			},
		},
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	return nil
}

func ExampleStackSpec() error {
	// This stack creates a bucket
	bucketProj := ProjectSpec{
		Name: "bucket_provider",
		// InlineSource is the function defining resources in the Pulumi project
		// this program creates an AWS S3 Bucket
		InlineSource: func(ctx *pulumi.Context) error {
			bucket, err := s3.NewBucket(ctx, "bucket", nil)
			if err != nil {
				return err
			}
			ctx.Export("bucketName", bucket.Bucket)
			return nil
		},
	}
	// a stack spec descripes an instance of a Pulumi program
	bucketStack := StackSpec{
		Name:    "dev_bucket",
		Project: bucketProj,
		// overrides corresponding to pulumi.<stack>.yaml
		Overrides: &StackOverrides{
			// define config to be used in the stack
			Config: map[string]string{"aws:region": "us-west-2"},
			ProjectStack: &workspace.ProjectStack{
				// specify a user managed key
				EncryptedKey: "bring your own encryption key",
			},
		},
	}

	// initialize an instance of the stack
	s, err := NewStack(bucketStack)
	if err != nil {
		return err
	}

	return nil
}

func ExampleStack() error {
	ps := ProjectSpec{
		Name: "proj",
		InlineSource: func(ctx *pulumi.Context) error {
			c := config.New(ctx, "")
			ctx.Export("exp_static", pulumi.String("foo"))
			ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
			ctx.Export("exp_secret", c.GetSecret("buzz"))
			return nil
		},
	}
	ss := StackSpec{
		Name:    "dev",
		Project: ps,
		Overrides: &StackOverrides{
			Config:  map[string]string{"bar": "abc"},
			Secrets: map[string]string{"buzz": "secret"},
		},
	}

	// initialize, selecting an existing stack or creating a new one if none exists
	s, err := NewStack(ss)
	if err != nil {
		return err
	}

	// deploy the stack
	_, err = s.Up()
	if err != nil {
		return err
	}

	// preview any pending changes
	_, err = s.Preview()
	if err != nil {
		return err
	}

	// refresh the resources in a stack, reading the state of the world directly from cloud providers
	_, err = s.Refresh()

	if err != nil {
		return err
	}

	// delete all resources in the stack
	_, err = s.Destroy()
	if err != nil {
		return err
	}

	// remove the stack and all associated configuration and history
	err = s.Remove()
	return err
}

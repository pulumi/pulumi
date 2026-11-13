package main

import (
	"example.com/pulumi-unionref/sdk/go/v63/unionref"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		github, err := unionref.NewGithub(ctx, "github", nil)
		if err != nil {
			return err
		}
		gitlab, err := unionref.NewGitlab(ctx, "gitlab", nil)
		if err != nil {
			return err
		}
		_, err = unionref.NewPipeline(ctx, "pipeline", &unionref.PipelineArgs{
			Source: github,
			Program: &unionref.ProgramArgs{
				Source: gitlab,
			},
			Sources: pulumi.Array{
				github,
				gitlab,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}

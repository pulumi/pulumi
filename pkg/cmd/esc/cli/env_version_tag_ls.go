// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvVersionTagLsCmd(env *envCommand) *cobra.Command {
	var pagerFlag string
	var utc bool
	var output string

	cmd := &cobra.Command{
		Use:   "ls [<org-name>/][<project-name>/]<environment-name>",
		Short: "List tagged versions.",
		Long: "List tagged versions\n" +
			"\n" +
			"This command lists an environment's tagged versions.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the ls command does not accept versions")
			}
			_ = args

			allTags, err := listAllEnvironmentRevisionTags(ctx, env.esc.client, ref)
			if err != nil {
				return err
			}

			if format == outputJSON {
				out := struct {
					Tags []revisionTagJSON `json:"tags"`
				}{Tags: make([]revisionTagJSON, 0, len(allTags))}
				for _, t := range allTags {
					out.Tags = append(out.Tags, revisionTagJSON{
						Name:        t.Name,
						Revision:    t.Revision,
						Modified:    utcFlag(utc).time(t.Modified),
						EditorLogin: t.EditorLogin,
						EditorName:  t.EditorName,
					})
				}
				return writeJSON(env.esc.stdout, out)
			}

			return env.esc.pager.Run(pagerFlag, env.esc.stdout, env.esc.stderr, func(ctx context.Context, stdout io.Writer) error {
				if len(allTags) == 0 {
					return nil
				}
				t := newTable(stdout)
				t.AppendHeader(table.Row{"NAME", "REVISION", "MODIFIED", "EDITOR"})
				for _, tag := range allTags {
					t.AppendRow(table.Row{
						tag.Name,
						tag.Revision,
						utcFlag(utc).time(tag.Modified).String(),
						revisionTagEditor(tag),
					})
				}
				t.Render()
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&pagerFlag, "pager", "", "the command to use to page through the environment's version tags")
	cmd.Flags().BoolVar(&utc, "utc", false, "display times in UTC")
	addOutputFlag(cmd, &output)

	return cmd
}

// revisionTagJSON is the slim per-tag projection emitted by JSON output.
// Mirrors the fields shown in the text table; `created` is omitted because
// text only shows `modified`.
type revisionTagJSON struct {
	Name        string    `json:"name"`
	Revision    int       `json:"revision"`
	Modified    time.Time `json:"modified"`
	EditorLogin string    `json:"editorLogin,omitempty"`
	EditorName  string    `json:"editorName,omitempty"`
}

// listAllEnvironmentRevisionTags pages through every revision tag on the environment.
func listAllEnvironmentRevisionTags(
	ctx context.Context,
	c client.Client,
	ref environmentRef,
) ([]client.EnvironmentRevisionTag, error) {
	all := []client.EnvironmentRevisionTag{}
	after := ""
	count := 500
	for {
		options := client.ListEnvironmentRevisionTagsOptions{
			After: after,
			Count: &count,
		}
		tags, err := c.ListEnvironmentRevisionTags(ctx, ref.orgName, ref.projectName, ref.envName, options)
		if err != nil {
			return nil, err
		}
		if len(tags) == 0 {
			return all, nil
		}
		after = tags[len(tags)-1].Name
		all = append(all, tags...)
	}
}

func revisionTagEditor(t client.EnvironmentRevisionTag) string {
	if t.EditorLogin == "" {
		return "<unknown>"
	}
	return fmt.Sprintf("%s <%s>", t.EditorName, t.EditorLogin)
}

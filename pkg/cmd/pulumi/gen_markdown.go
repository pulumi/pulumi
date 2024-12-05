// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// Used to replace the `## <command>` line in generated markdown files.
var replaceH2Pattern = regexp.MustCompile(`(?m)^## .*$`)

// Used to promote the `###` headings to `##` in generated markdown files.
var h3Pattern = regexp.MustCompile(`(?m)^###\s`)

// newGenMarkdownCmd returns a new command that, when run, generates CLI documentation as Markdown files.
// It is hidden by default since it's not commonly used outside of our own build processes.
func newGenMarkdownCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-markdown <DIR>",
		Args:   cmdutil.ExactArgs(1),
		Short:  "Generate Pulumi CLI documentation as Markdown (one file per command)",
		Hidden: true,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			var files []string

			// filePrepender is used to add front matter to each file, and to keep track of all
			// generated files.
			filePrepender := func(s string) string {
				// Keep track of the generated file.
				files = append(files, s)

				// Add some front matter to each file.
				fileNameWithoutExtension := strings.TrimSuffix(filepath.Base(s), ".md")
				title := strings.ReplaceAll(fileNameWithoutExtension, "_", " ")
				ymlIndent := "  " // 2 spaces
				buf := new(bytes.Buffer)
				buf.WriteString("---\n")
				fmt.Fprintf(buf, "title: %q\n", title)
				// Add redirect aliases to the front matter.
				fmt.Fprint(buf, "aliases:\n")
				fmt.Fprintf(buf, "%s- /docs/reference/cli/%s/\n", ymlIndent, fileNameWithoutExtension)
				buf.WriteString("---\n\n")
				return buf.String()
			}

			// linkHandler emits pretty URL links.
			linkHandler := func(s string) string {
				link := strings.TrimSuffix(s, ".md")
				return fmt.Sprintf("/docs/iac/cli/commands/%s/", link)
			}

			// Generate the .md files.
			if err := doc.GenMarkdownTreeCustom(root, args[0], filePrepender, linkHandler); err != nil {
				return err
			}

			// Now loop through each generated file and replace the `## <command>` line, since
			// we're already adding the name of the command as a title in the front matter.
			for _, file := range files {
				b, err := os.ReadFile(file)
				if err != nil {
					return err
				}

				// Replace the `## <command>` line with an empty string.
				// We do this because we're already including the command as the front matter title.
				result := replaceH2Pattern.ReplaceAllString(string(b), "")

				// Promote the `###` to `##` headings. We removed the command name above which was
				// a level 2 heading (##), so need to promote the ### to ## so there is no gap in
				// heading levels when these files are used to render the CLI docs on the docs site.
				result = h3Pattern.ReplaceAllString(result, "## ")

				if err := os.WriteFile(file, []byte(result), 0o600); err != nil {
					return err
				}
			}

			return nil
		}),
	}
}

// Copyright 2016-2021, Pulumi Corporation.
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
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/alecthomas/chroma"
	"github.com/pgavlin/goldmark"
	goldmark_renderer "github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/text"
	"github.com/pgavlin/goldmark/util"
	"github.com/pgavlin/markdown-kit/renderer"
	"github.com/pgavlin/markdown-kit/styles"
	_ "github.com/pgavlin/svg2"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newStackReadmeCmd() *cobra.Command {
	var stackName string
	var showImages bool
	var wrap int

	cmd := &cobra.Command{
		Use:   "readme",
		Args:  cmdutil.NoArgs,
		Short: "Display the current stack's README, if any",
		Long: "Display the current stack's README, if any.\n" +
			"\n" +
			"This command renders the current stack's README to standard out. A stack's README\n" +
			"is pulled from the value of its `readme` output. If such an output exists, its\n" +
			"value is interpreted as Markdown and rendered to the terminal.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stackName, false, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}
			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			// This will prevent a panic when trying to assemble a dependencyGraph when no snapshot is found
			if snap == nil {
				return fmt.Errorf("unable to find snapshot for stack %q", stackName)
			}

			readme, hasREADME, err := getREADME(snap)
			if err != nil {
				return err
			}
			if !hasREADME {
				fmt.Printf("stack %q has no readme\n", stackName)
				return nil
			}

			projectPath, err := workspace.DetectProjectPath()
			if err != nil {
				return err
			}
			projectDir := filepath.Dir(projectPath)

			return renderREADME(os.Stdout, []byte(readme), showImages, wrap, projectDir)
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVar(&showImages, "show-images", true,
		"True to display images on supported terminals (currently only kitty)")
	cmd.PersistentFlags().IntVar(&wrap, "wrap", 0, "The maximum line width for wrappable content")
	return cmd
}

var readmeSubst = regexp.MustCompile(`.?\${.*?}`)

func getREADME(snap *deploy.Snapshot) (string, bool, error) {
	stackResource, err := stack.GetRootStackResource(snap)
	if err != nil {
		return "", false, err
	}
	if stackResource == nil {
		return "", false, nil
	}

	readmeV, ok := stackResource.Outputs["readme"]
	if !ok || !readmeV.IsString() {
		return "", false, nil
	}
	readme := readmeV.StringValue()

	context := resource.NewObjectProperty(resource.PropertyMap{
		"outputs": resource.NewObjectProperty(stackResource.Outputs),
	})

	getPropertyString := func(pathStr string) (string, error) {
		path, err := resource.ParsePropertyPath(pathStr)
		if err != nil {
			return "", err
		}

		v, ok := path.Get(context)
		if !ok {
			return "", nil
		}
		switch {
		case v.IsNull():
			return "null", nil
		case v.IsBool(), v.IsNumber(), v.IsString():
			return fmt.Sprintf("%v", v.V), nil
		case v.IsSecret():
			return "<secret>", nil
		case v.IsComputed(), v.IsOutput():
			return "<unknown>", nil
		case v.IsResourceReference():
			return string(v.ResourceReferenceValue().URN), nil
		default:
			return fmt.Sprintf("<%v>", v.TypeString()), nil
		}
	}

	readme = readmeSubst.ReplaceAllStringFunc(readme, func(s string) string {
		if err != nil {
			return s
		}

		// s is of the form $${....}, ${....} or ?${....}
		path, prefix := "", ""
		switch s[0:2] {
		case "$$":
			return s[1:]
		case "${":
			path = s[2 : len(s)-1]
		default:
			path, prefix = s[3:len(s)-1], s[:1]
		}

		sub, subErr := getPropertyString(path)
		if subErr != nil {
			err = subErr
			return s
		}

		return prefix + sub
	})
	if err != nil {
		return "", false, err
	}
	return readme, true, nil
}

func renderREADME(w io.Writer, source []byte, showImages bool, wrap int, projectDirectory string) error {
	var termWidth int
	var theme *chroma.Style
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		theme = styles.Pulumi

		termSupportsImages, width, _ := canDisplayImages()
		if termWidth == 0 {
			w, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil {
				width = int(w)
			}
		}

		if showImages {
			showImages = termSupportsImages
		}
		if wrap == 0 {
			wrap = termWidth
		}
		termWidth = width
	}

	parser := goldmark.DefaultParser()
	document := parser.Parse(text.NewReader(source))

	r := renderer.New(
		renderer.WithTheme(theme),
		renderer.WithWordWrap(wrap),
		renderer.WithSoftBreak(wrap != 0),
		renderer.WithImages(showImages, termWidth, projectDirectory))
	renderer := goldmark_renderer.NewRenderer(goldmark_renderer.WithNodeRenderers(util.Prioritized(r, 100)))
	return renderer.Render(os.Stdout, source, document)
}

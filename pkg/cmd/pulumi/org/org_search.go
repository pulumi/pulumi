// Copyright 2016, Pulumi Corporation.
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

package org

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Delimiter rune

func (d *Delimiter) String() string {
	return string(*d)
}

func (d *Delimiter) Set(v string) error {
	if v == "\\t" {
		*d = Delimiter('\t')
	} else if len(v) != 1 {
		return errors.New("delimiter must be a single character")
	} else {
		*d = Delimiter(v[0])
	}
	return nil
}

func (d *Delimiter) Type() string {
	return "Delimiter"
}

func (d *Delimiter) Rune() rune {
	return rune(*d)
}

// searchRender renders a search response to cmd.Stdout. The receiver is passed
// explicitly so we can capture per-format renderers as function values.
type searchRender func(cmd *searchCmd, results *apitype.ResourceSearchResponse) error

// defaultSearchOutputFormat returns the OutputFlag wired up with every supported
// format. Callers (cobra constructors and tests) install this on searchCmd so
// `--output` selects between them.
func defaultSearchOutputFormat() outputflag.OutputFlag[searchRender] {
	return outputflag.OutputFlag[searchRender]{
		RenderForTerminal: (*searchCmd).RenderTable,
		RenderJSON:        (*searchCmd).RenderJSON,
		RenderYAML:        (*searchCmd).RenderYAML,
		RenderCSV: func(cmd *searchCmd, results *apitype.ResourceSearchResponse) error {
			return cmd.RenderCSV(results.Resources, cmd.csvDelimiter.Rune())
		},
	}
}

type searchCmd struct {
	orgName      string
	csvDelimiter Delimiter
	outputFormat outputflag.OutputFlag[searchRender]
	openWeb      bool

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

type orgSearchCmd struct {
	searchCmd
	queryParams []string
}

func (cmd *orgSearchCmd) Run(ctx context.Context, args []string) error {
	interactive := cmdutil.Interactive()

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	displayOpts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
	}
	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	currentBe, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return err
	}
	cloudBackend, isCloud := currentBe.(httpstate.Backend)
	if !isCloud {
		return errors.New("Pulumi AI search is only supported for the Pulumi Cloud")
	}
	defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
	if err != nil {
		return err
	}
	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return err
	}
	if defaultOrg != "" && cmd.orgName == "" {
		cmd.orgName = defaultOrg
	}
	if cmd.orgName != "" {
		if !slices.Contains(orgs, cmd.orgName) {
			return fmt.Errorf("user %s is not a member of organization %s", userName, cmd.orgName)
		}
	} else {
		cmd.orgName = userName
	}

	parsedQueryParams := apitype.ParseQueryParams(cmd.queryParams)
	res, err := cloudBackend.Search(ctx, cmd.orgName, parsedQueryParams)
	if err != nil {
		return err
	}
	err = cmd.outputFormat.Get()(&cmd.searchCmd, res)
	if err != nil {
		return fmt.Errorf("table rendering error: %w", err)
	}
	if cmd.openWeb {
		err = browser.OpenURL(res.URL)
		if err != nil {
			logging.Warningf("failed to open URL: %s", err)
		}
	}
	return nil
}

func newSearchCmd() *cobra.Command {
	var scmd orgSearchCmd
	scmd.outputFormat = defaultSearchOutputFormat()
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for resources in Pulumi Cloud",
		Long:  "Search for resources in Pulumi Cloud.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(scmd.queryParams) == 0 {
				return cmd.Help()
			}
			return scmd.Run(ctx, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newSearchAICmd())

	cmd.PersistentFlags().StringVar(
		&scmd.orgName, "org", "",
		"Name of the organization to search. Defaults to the current user's default organization.",
	)
	cmd.PersistentFlags().StringArrayVarP(
		&scmd.queryParams, "query", "q", nil,
		"A Pulumi Query to send to Pulumi Cloud for resource search."+
			"May be formatted as a single query, or multiple:\n"+
			"\t-q \"type:aws:s3/bucketv2:BucketV2 modified:>=2023-09-01\"\n"+
			"\t-q \"type:aws:s3/bucketv2:BucketV2\" -q \"modified:>=2023-09-01\"\n"+
			"See https://www.pulumi.com/docs/pulumi-cloud/insights/search/#query-syntax for syntax reference.",
	)
	outputflag.Var(cmd.PersistentFlags(), &scmd.outputFormat)
	cmd.PersistentFlags().Var(
		&scmd.csvDelimiter, "delimiter",
		"Delimiter to use when rendering CSV output.",
	)
	cmd.PersistentFlags().BoolVar(
		&scmd.openWeb, "web", false,
		"Open the search results in a web browser.",
	)

	return cmd
}

func renderSearchTable(w io.Writer, results *apitype.ResourceSearchResponse) error {
	urlInfo := "Results are also visible in Pulumi Cloud:"
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	// Preserve title-case headers; the default StyleLight upper-cases them.
	t.Style().Format.Header = text.FormatDefault
	t.AppendHeader(table.Row{"Project", "Stack", "Name", "Type", "Package", "Module", "Modified"})
	for _, r := range results.Resources {
		t.AppendRow(table.Row{*r.Program, *r.Stack, *r.Name, *r.Type, *r.Package, *r.Module, *r.Modified})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Name: "Stack", WidthMax: 30, WidthMaxEnforcer: text.WrapText},
		{Name: "Name", WidthMax: 30, WidthMaxEnforcer: text.WrapText},
	})
	t.Render()
	_, err := fmt.Fprintf(w,

		"Displaying %s of %s total results.\n",
		strconv.Itoa(len(results.Resources)),
		strconv.FormatInt(*results.Total, 10))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(urlInfo + "\n" + results.URL + "\n"))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w,
		"\nResources displayed are the result of the following Pulumi query:\n%s\n", results.Query)
	return err
}

func (cmd *searchCmd) RenderTable(results *apitype.ResourceSearchResponse) error {
	return renderSearchTable(cmd.Stdout, results)
}

func renderSearchCSV(w io.Writer, results []apitype.ResourceResult, delimiter rune) error {
	data := make([][]string, 0, len(results)+1)
	writer := csv.NewWriter(w)
	if delimiter != 0 {
		writer.Comma = delimiter
	}
	data = append(data, []string{"Project", "Stack", "Name", "Type", "Package", "Module", "Modified"})
	for _, result := range results {
		data = append(data, []string{
			*result.Program,
			*result.Stack,
			*result.Name,
			*result.Type,
			*result.Package,
			*result.Module,
			*result.Modified,
		})
	}
	return writer.WriteAll(data)
}

func (cmd *searchCmd) RenderCSV(results []apitype.ResourceResult, delimiter rune) error {
	return renderSearchCSV(cmd.Stdout, results, delimiter)
}

func renderSearchJSON(w io.Writer, results []apitype.ResourceResult) error {
	output, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		return err
	}
	_, err = w.Write(output)
	return err
}

func (cmd *searchCmd) RenderJSON(result *apitype.ResourceSearchResponse) error {
	return renderSearchJSON(cmd.Stdout, result.Resources)
}

func renderSearchYAML(w io.Writer, results []apitype.ResourceResult) error {
	output, err := yaml.Marshal(results)
	if err != nil {
		return err
	}
	_, err = w.Write(output)
	return err
}

func (cmd *searchCmd) RenderYAML(result *apitype.ResourceSearchResponse) error {
	return renderSearchYAML(cmd.Stdout, result.Resources)
}

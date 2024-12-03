// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	auto_table "go.pennock.tech/tabular/auto"
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

type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
	outputFormatYAML  outputFormat = "yaml"
	outputFormatCSV   outputFormat = "csv"
)

// String is used both by fmt.Print and by Cobra in help text
func (o *outputFormat) String() string {
	return string(*o)
}

// Set must have pointer receiver so it doesn't change the value of a copy
func (o *outputFormat) Set(v string) error {
	switch v {
	case "csv", "table", "json", "yaml":
		*o = outputFormat(v)
		return nil
	default:
		return errors.New(`must be one of "csv", "table", "json", or "yaml"`)
	}
}

// Type is only used in help text
func (o *outputFormat) Type() string {
	return "outputFormat"
}

type searchCmd struct {
	orgName      string
	csvDelimiter Delimiter
	outputFormat
	openWeb bool

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, backend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

type orgSearchCmd struct {
	searchCmd
	queryParams []string
}

func (cmd *orgSearchCmd) Run(ctx context.Context, args []string) error {
	interactive := cmdutil.Interactive()

	if cmd.outputFormat == "" {
		cmd.outputFormat = outputFormatTable
	}

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := backend.QueryOptions{}
	opts.Display = display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
		Type:          display.DisplayQuery,
	}
	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	backend, err := currentBackend(ctx, ws, DefaultLoginManager, project, opts.Display)
	if err != nil {
		return err
	}
	cloudBackend, isCloud := backend.(httpstate.Backend)
	if !isCloud {
		return errors.New("Pulumi AI search is only supported for the Pulumi Cloud")
	}
	defaultOrg, err := pkgWorkspace.GetBackendConfigDefaultOrg(project)
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
		if !sliceContains(orgs, cmd.orgName) {
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
	err = cmd.outputFormat.Render(&cmd.searchCmd, res)
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
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for resources in Pulumi Cloud",
		Long:  "Search for resources in Pulumi Cloud.",
		Args:  cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(scmd.queryParams) == 0 {
				return cmd.Help()
			}
			return scmd.Run(ctx, args)
		},
		),
	}

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
	cmd.PersistentFlags().VarP(
		&scmd.outputFormat, "output", "o",
		"Output format. Supported formats are 'table', 'json', 'csv', and 'yaml'.",
	)
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

func sliceContains(slice []string, search string) bool {
	for _, s := range slice {
		if s == search {
			return true
		}
	}
	return false
}

func renderSearchTable(w io.Writer, results *apitype.ResourceSearchResponse) error {
	urlInfo := "Results are also visible in Pulumi Cloud:"
	table := auto_table.New("utf8-heavy")
	table.AddHeaders("Project", "Stack", "Name", "Type", "Package", "Module", "Modified")
	for _, r := range results.Resources {
		table.AddRowItems(*r.Program, *r.Stack, *r.Name, *r.Type, *r.Package, *r.Module, *r.Modified)
	}
	if errs := table.Errors(); errs != nil {
		return errors.Join(errs...)
	}
	err := table.RenderTo(w)
	if err != nil {
		return err
	}
	_, err = w.Write(
		[]byte(
			fmt.Sprintf(
				"Displaying %s of %s total results.\n",
				strconv.Itoa(len(results.Resources)),
				strconv.FormatInt(*results.Total, 10))),
	)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(urlInfo + "\n" + results.URL + "\n"))
	if err != nil {
		return err
	}
	_, err = w.Write(
		[]byte(fmt.Sprintf("\nResources displayed are the result of the following Pulumi query:\n%s\n", results.Query)),
	)
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

func (o *outputFormat) Render(cmd *searchCmd, result *apitype.ResourceSearchResponse) error {
	switch *o {
	case outputFormatJSON:
		return cmd.RenderJSON(result)
	case outputFormatTable:
		return cmd.RenderTable(result)
	case outputFormatYAML:
		return cmd.RenderYAML(result)
	case outputFormatCSV:
		return cmd.RenderCSV(result.Resources, cmd.csvDelimiter.Rune())
	default:
		return fmt.Errorf("unknown output format %q", *o)
	}
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

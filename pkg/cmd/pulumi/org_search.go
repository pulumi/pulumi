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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	auto_table "go.pennock.tech/tabular/auto"
	"gopkg.in/yaml.v3"
)

//nolint:lll
type OrgSearchArgs struct {
	Organization   string       `args:"org" argsUsage:"Name of the organization to search. Defaults to the current user's default organization."`
	CSVDelimiter   Delimiter    `args:"delimiter" argsUsage:"Delimiter to use when rendering CSV output."`
	OutputFormat   outputFormat `args:"output" argsType:"var" argsShort:"o" argsUsage:"Output format. Supported formats are 'table', 'json', 'csv', and 'yaml'."`
	OpenWebBrowser bool         `args:"web" argsUsage:"Open the search results in a web browser."`
	QueryParams    []string     `args:"query" argsShort:"q" argsUsage:"A Pulumi Query to send to Pulumi Cloud for resource search. May be formatted as a single query, or multiple:\n\t-q \"type:aws:s3/bucket:Bucket modified:>=2023-09-01\"\n\t-q \"type:aws:s3/bucket:Bucket\" -q \"modified:>=2023-09-01\"\nSee https://www.pulumi.com/docs/pulumi-cloud/insights/search/#query-syntax for syntax reference."`
}

type orgSearchCmd struct {
	Args OrgSearchArgs

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func newSearchCmd(
	v *viper.Viper,
	parentOrgCmd *cobra.Command,
) *cobra.Command {
	var scmd orgSearchCmd
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for resources in Pulumi Cloud",
		Long:  "Search for resources in Pulumi Cloud.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			scmd.Args = UnmarshalArgs[OrgSearchArgs](v, cmd)

			ctx := cmd.Context()
			if len(scmd.Args.QueryParams) == 0 {
				return cmd.Help()
			}
			return scmd.Run(ctx, cliArgs)
		},
		),
	}

	parentOrgCmd.AddCommand(cmd)
	BindFlags[OrgSearchArgs](v, cmd)

	newSearchAICmd(v, cmd)

	return cmd
}

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

func (cmd *orgSearchCmd) Run(ctx context.Context, args []string) error {
	interactive := cmdutil.Interactive()

	if cmd.Args.OutputFormat == "" {
		cmd.Args.OutputFormat = outputFormatTable
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
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	backend, err := currentBackend(ctx, project, opts.Display)
	if err != nil {
		return err
	}
	cloudBackend, isCloud := backend.(httpstate.Backend)
	if !isCloud {
		return errors.New("Pulumi AI search is only supported for the Pulumi Cloud")
	}
	defaultOrg, err := workspace.GetBackendConfigDefaultOrg(project)
	if err != nil {
		return err
	}
	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return err
	}
	if defaultOrg != "" && cmd.Args.Organization == "" {
		cmd.Args.Organization = defaultOrg
	}
	if cmd.Args.Organization != "" {
		if !sliceContains(orgs, cmd.Args.Organization) {
			return fmt.Errorf("user %s is not a member of organization %s", userName, cmd.Args.Organization)
		}
	} else {
		cmd.Args.Organization = userName
	}

	parsedQueryParams := apitype.ParseQueryParams(cmd.Args.QueryParams)
	res, err := cloudBackend.Search(ctx, cmd.Args.Organization, parsedQueryParams)
	if err != nil {
		return err
	}
	err = cmd.Args.OutputFormat.Render(cmd.Stdout, rune(cmd.Args.CSVDelimiter), res)
	if err != nil {
		return fmt.Errorf("table rendering error: %w", err)
	}
	if cmd.Args.OpenWebBrowser {
		err = browser.OpenURL(res.URL)
		if err != nil {
			logging.Warningf("failed to open URL: %s", err)
		}
	}
	return nil
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

func renderSearchJSON(w io.Writer, results []apitype.ResourceResult) error {
	output, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		return err
	}
	_, err = w.Write(output)
	return err
}

func (o *outputFormat) Render(w io.Writer, csvDelimiter rune, result *apitype.ResourceSearchResponse) error {
	switch *o {
	case outputFormatJSON:
		return renderSearchJSON(w, result.Resources)
	case outputFormatTable:
		return renderSearchTable(w, result)
	case outputFormatYAML:
		return renderSearchYAML(w, result.Resources)
	case outputFormatCSV:
		return renderSearchCSV(w, result.Resources, csvDelimiter)
	default:
		return fmt.Errorf("unknown output format %q", *o)
	}
}

func renderSearchYAML(w io.Writer, results []apitype.ResourceResult) error {
	output, err := yaml.Marshal(results)
	if err != nil {
		return err
	}
	_, err = w.Write(output)
	return err
}

// Copyright 2026, Pulumi Corporation.
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

package policy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v3"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// scanDriverJS runs the program against a recording resource monitor and writes the resources it registers.
//
//go:embed scan/scan.js
var scanDriverJS []byte

// scanFixturesFile is the default fixtures file, next to Pulumi.yaml.
const scanFixturesFile = "PulumiScan.yaml"

// scanGraph is what the driver records: the resources the program registered, and what it had to make up.
type scanGraph struct {
	Project    string           `json:"project"`
	Stack      string           `json:"stack"`
	ProgramDir string           `json:"programDir"`
	CodeRoot   string           `json:"codeRoot"`
	Resources  []scanResource   `json:"resources"`
	Stubs      []string         `json:"stubs"`
	Missing    []string         `json:"missingConfig"`
	Assumed    []string         `json:"assumedArtifacts"`
	Blocked    []string         `json:"networkBlocked"`
	Fixtures   []string         `json:"usedFixtures"`
	Unmocked   []string         `json:"unknownInvokes"`
	Probed     int              `json:"probedApplies"`
	Fallbacks  int              `json:"failedApplies"`
	Errors     []scanMessage    `json:"errors"`
	Timings    map[string]int64 `json:"timings"`
}

type scanMessage struct {
	Message string `json:"message"`
}

type scanPosition struct {
	URI    string `json:"uri"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type scanResource struct {
	URN                  string              `json:"urn"`
	Type                 string              `json:"type"`
	Name                 string              `json:"name"`
	Parent               string              `json:"parent"`
	Dependencies         []string            `json:"dependencies"`
	PropertyDependencies map[string][]string `json:"propertyDependencies"`
	SourcePosition       *scanPosition       `json:"sourcePosition"`
	Object               map[string]any      `json:"object"`
}

// scanFinding is one policy result tied to the line of code that declared the resource. Its fields map onto an
// LSP diagnostic or a GitHub check annotation.
type scanFinding struct {
	URI              string `json:"uri,omitempty"`
	File             string `json:"file,omitempty"`
	Line             int    `json:"line,omitempty"`
	Column           int    `json:"column,omitempty"`
	URN              string `json:"urn"`
	Type             string `json:"type"`
	Name             string `json:"name"`
	PolicyPack       string `json:"policyPack"`
	PolicyPackVer    string `json:"policyPackVersion"`
	Policy           string `json:"policy"`
	Description      string `json:"description,omitempty"`
	Severity         string `json:"severity,omitempty"`
	EnforcementLevel string `json:"enforcementLevel"`
	Message          string `json:"message"`
}

type scanReport struct {
	Project    string        `json:"project"`
	ProgramDir string        `json:"programDir"`
	Resources  int           `json:"resources"`
	Findings   []scanFinding `json:"findings"`
	Undecided  []scanFinding `json:"undecided"`
	Stubs      []string      `json:"stubs"`
	Missing    []string      `json:"missingConfig"`
	Assumed    []string      `json:"assumedArtifacts"`
	Blocked    []string      `json:"blocked"`
	Fixtures   []string      `json:"usedFixtures"`
	Unmocked   []string      `json:"unknownWithoutFixture"`
	Probed     int           `json:"probedApplies"`
	Fallbacks  int           `json:"failedApplies"`
	// Linked and Unlinked count unknown inputs that are another resource's output (or built from outputs), and
	// those that aren't.
	Linked   int              `json:"identifiableUnknowns"`
	Unlinked int              `json:"otherUnknowns"`
	Errors   []string         `json:"errors"`
	Timings  map[string]int64 `json:"timings"`
}

type loadAnalyzersFunc func(ctx context.Context, packs []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error)

func newPolicyScanCmd(loadAnalyzers loadAnalyzersFunc) *cobra.Command {
	var policyPackPaths []string
	var policyPackConfigs []string
	var stackName string
	var configOverrides []string
	var fixturesFile string
	var jsonOut bool
	var sdkDir string

	cmd := &cobra.Command{
		Use:    "scan [dir]",
		Hidden: true,
		Short:  "[EXPERIMENTAL] Check a program's code against policy packs, without a preview",
		Long: "[EXPERIMENTAL] Check a Pulumi program's code against local policy packs.\n" +
			"\n" +
			"The program runs with mocks, the way unit tests run it: no engine, providers, cloud\n" +
			"credentials, login or stack are involved, and the program's dependencies don't need\n" +
			"to be installed. Inputs are what the program declares; outputs a provider would fill\n" +
			"in are unknown. Each finding points to the line of code that declared the resource.\n" +
			"\n" +
			"Data source results, other stacks' outputs and config can be given in " + scanFixturesFile + "\n" +
			"next to Pulumi.yaml. Network access and child processes are blocked.\n" +
			"\n" +
			"Only Node.js programs are supported. Exits with a non-zero status if any mandatory\n" +
			"violations are found.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			start := time.Now()
			if len(policyPackPaths) == 0 {
				return errors.New("at least one --policy-pack path must be specified")
			}
			if len(policyPackConfigs) > 0 && len(policyPackConfigs) != len(policyPackPaths) {
				return errors.New(
					"the number of --policy-pack-config paths must match the number of --policy-pack paths")
			}
			if loadAnalyzers == nil {
				loadAnalyzers = loadScanAnalyzers
			}

			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			dir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			projPath, err := workspace.DetectProjectPathFrom(dir)
			if err != nil || projPath == "" {
				return fmt.Errorf("no Pulumi.yaml found in %s or its parents", dir)
			}
			proj, err := workspace.LoadProject(projPath)
			if err != nil {
				return err
			}
			projectDir := filepath.Dir(projPath)
			if rt := proj.Runtime.Name(); rt != "nodejs" {
				return fmt.Errorf("policy scan supports only Node.js programs; %s uses %q", projectDir, rt)
			}
			programDir, mainFile := projectDir, ""
			// `main` is the program's directory or its entry file.
			if proj.Main != "" {
				p := filepath.Join(projectDir, proj.Main)
				if fi, err := os.Stat(p); err == nil && fi.IsDir() {
					programDir = p
				} else {
					mainFile = proj.Main
				}
			}

			home, err := workspace.GetPulumiHomeDir()
			if err != nil {
				return err
			}
			cacheDir := filepath.Join(home, "policy-scan", "cache")
			sdkDir, err = resolveScanSDK(ctx, cmd.ErrOrStderr(), sdkDir, cacheDir)
			if err != nil {
				return err
			}

			tmp, err := os.MkdirTemp("", "pulumi-policy-scan-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)

			if fixturesFile == "" {
				fixturesFile = filepath.Join(projectDir, scanFixturesFile)
			}
			fixtures, err := readScanFixtures(fixturesFile)
			if err != nil {
				return err
			}
			// The fixtures file can name the stack to scan as, so editors and CI need no flags.
			if !cmd.Flags().Changed("stack") && fixtures.Stack != "" {
				stackName = fixtures.Stack
			}
			configFile, err := writeScanConfig(tmp, proj, projectDir, stackName, configOverrides, fixtures)
			if err != nil {
				return err
			}

			// Load the packs while the program runs.
			type loaded struct {
				analyzers []plugin.Analyzer
				cleanup   func()
				err       error
				took      time.Duration
			}
			loadCh := make(chan loaded, 1)
			go func() {
				t := time.Now()
				as, cleanup, err := loadAnalyzers(ctx, engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigs))
				loadCh <- loaded{as, cleanup, err, time.Since(t)}
			}()

			scanStart := time.Now()
			graph, err := runScanDriver(ctx, cmd.ErrOrStderr(), tmp, scanDriverArgs{
				programDir: programDir,
				sdkDir:     sdkDir,
				cacheDir:   cacheDir,
				project:    string(proj.Name),
				stack:      stackName,
				configFile: configFile,
				mainFile:   mainFile,
			})
			scanTook := time.Since(scanStart)
			l := <-loadCh
			if l.cleanup != nil {
				defer l.cleanup()
			}
			if err != nil {
				return err
			}
			if l.err != nil {
				return fmt.Errorf("loading policy packs: %w", l.err)
			}

			analyzeStart := time.Now()
			report, err := analyzeScanGraph(ctx, graph, l.analyzers)
			if err != nil {
				return err
			}
			report.Timings = map[string]int64{
				"programMs":  scanTook.Milliseconds(),
				"loadPackMs": l.took.Milliseconds(),
				"analyzeMs":  time.Since(analyzeStart).Milliseconds(),
				"totalMs":    time.Since(start).Milliseconds(),
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printScanReport(cmd.OutOrStdout(), report)
			}

			for _, f := range report.Findings {
				if f.EnforcementLevel == string(apitype.Mandatory) {
					return errors.New("one or more mandatory policy violations were found")
				}
			}
			if len(report.Errors) > 0 {
				return errors.New("the program didn't finish; findings may be incomplete")
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&policyPackPaths, "policy-pack", nil, "Path to a policy pack to run")
	cmd.Flags().StringArrayVar(&policyPackConfigs, "policy-pack-config", nil,
		"Path to a JSON config file for the corresponding --policy-pack")
	cmd.Flags().StringArrayVarP(&configOverrides, "config", "c", nil,
		"Config value to use, as key=value (the namespace defaults to the project); overrides other config")
	cmd.Flags().StringVar(&fixturesFile, "fixtures", "",
		"Fixtures file (config, invokes, stackReferences); defaults to "+scanFixturesFile+" next to Pulumi.yaml")
	cmd.Flags().StringVarP(&stackName, "stack", "s", "scan",
		"Stack name to scan as; its Pulumi.<stack>.yaml config is used if present")
	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Emit the findings as JSON")
	cmd.Flags().StringVar(&sdkDir, "sdk", "",
		"Directory of the @pulumi/pulumi package to run the program with (defaults to the CLI's version)")
	return cmd
}

// releaseVersion matches a released CLI version, whose @pulumi/pulumi of the same version is on npm.
var releaseVersion = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)

// resolveScanSDK finds the @pulumi/pulumi the program runs with: --sdk, PULUMI_POLICY_SCAN_SDK, the SDK built next
// to a development CLI (bin/pulumi and sdk/nodejs/bin), or else the release of the CLI's own version, installed once
// into the cache. The program's own copy isn't used: it may predate scan support, or not be installed.
func resolveScanSDK(ctx context.Context, stderr io.Writer, flag, cacheDir string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if v := os.Getenv("PULUMI_POLICY_SCAN_SDK"); v != "" {
		return v, nil
	}
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			dev := filepath.Join(filepath.Dir(exe), "..", "sdk", "nodejs", "bin")
			if _, err := os.Stat(filepath.Join(dev, "runtime", "policyScan.js")); err == nil {
				return dev, nil
			}
		}
	}
	m := releaseVersion.FindStringSubmatch(version.Version)
	if m == nil {
		return "", fmt.Errorf("this CLI (version %q) is not a release: set --sdk or PULUMI_POLICY_SCAN_SDK to a "+
			"built @pulumi/pulumi", version.Version)
	}
	dir := filepath.Join(cacheDir, "pulumi", m[1])
	sdk := filepath.Join(dir, "node_modules", "@pulumi", "pulumi")
	if _, err := os.Stat(filepath.Join(sdk, "runtime", "policyScan.js")); err == nil {
		return sdk, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	fmt.Fprintf(stderr, "policy scan: installing @pulumi/pulumi@%s into %s\n", m[1], dir)
	install := exec.CommandContext(ctx, "npm", "install", "--prefix", dir, "--ignore-scripts", "--no-audit",
		"--no-fund", "--loglevel=error", "@pulumi/pulumi@"+m[1])
	install.Stdout, install.Stderr = stderr, stderr
	if err := install.Run(); err != nil {
		return "", fmt.Errorf("installing @pulumi/pulumi@%s: %w", m[1], err)
	}
	return sdk, nil
}

// scanFixtures is the fixtures file: config for the scan, and results for what only a deployment could give.
type scanFixtures struct {
	// Stack names the stack to scan as, unless --stack is given.
	Stack string `yaml:"stack"`
	// Config is applied after the stack's config and before --config, like a stack config file's.
	Config map[string]any `yaml:"config"`
	// Invokes are data source results, by full token or function name. Fields left out are unknown.
	Invokes map[string]any `yaml:"invokes"`
	// StackReferences are other stacks' outputs, by stack name (in full, or its trailing segments).
	StackReferences map[string]any `yaml:"stackReferences"`
}

func readScanFixtures(path string) (scanFixtures, error) {
	var f scanFixtures
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("reading fixtures %s: %w", path, err)
	}
	return f, nil
}

// scanConfigString renders a config value the way PULUMI_CONFIG carries it: strings as they are, the rest as JSON.
func scanConfigString(v any) (string, error) {
	if s, ok := v.(string); ok {
		return s, nil
	}
	j, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(j), nil
}

// writeScanConfig builds the program's config, in order: the project's defaults and values, the template's
// defaults, Pulumi.<stack>.yaml if present, the fixtures' config, then --config. Secure values can't be decrypted
// without the stack's secrets provider, so they become a placeholder secret. Values still missing get a placeholder
// from the SDK, which reports them.
func writeScanConfig(
	tmp string, proj *workspace.Project, projectDir, stack string, overrides []string, fixtures scanFixtures,
) (string, error) {
	values := map[string]string{}
	var secretKeys []string
	fullKey := func(k string) string {
		if !strings.Contains(k, ":") {
			return string(proj.Name) + ":" + k
		}
		return k
	}
	if proj.Template != nil {
		for k, v := range proj.Template.Config {
			if v.Default != "" {
				values[fullKey(k)] = v.Default
			}
		}
	}
	for k, v := range proj.Config {
		val := v.Value
		if val == nil {
			val = v.Default
		}
		if val == nil {
			continue
		}
		s, err := scanConfigString(val)
		if err != nil {
			return "", err
		}
		values[fullKey(k)] = s
	}
	apply := func(cfg map[string]any) error {
		for k, v := range cfg {
			k = fullKey(k)
			if v == nil {
				continue
			}
			if m, ok := v.(map[string]any); ok {
				if _, ok := m["secure"]; ok && len(m) == 1 {
					values[k] = "[secret]"
					secretKeys = append(secretKeys, k)
					continue
				}
			}
			s, err := scanConfigString(v)
			if err != nil {
				return err
			}
			values[k] = s
		}
		return nil
	}

	stackFile := filepath.Join(projectDir, "Pulumi."+stack+".yaml")
	if b, err := os.ReadFile(stackFile); err == nil {
		var doc struct {
			Config map[string]any `yaml:"config"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return "", fmt.Errorf("reading stack config %s: %w", stackFile, err)
		}
		if err := apply(doc.Config); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := apply(fixtures.Config); err != nil {
		return "", err
	}
	for _, kv := range overrides {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return "", fmt.Errorf("--config %q: expected key=value", kv)
		}
		values[fullKey(k)] = v
	}

	sort.Strings(secretKeys)
	out := filepath.Join(tmp, "config.json")
	j, err := json.Marshal(map[string]any{
		"values":     values,
		"secretKeys": secretKeys,
		"fixtures": map[string]any{
			"invokes":         orEmpty(fixtures.Invokes),
			"stackReferences": orEmpty(fixtures.StackReferences),
		},
	})
	if err != nil {
		return "", err
	}
	return out, os.WriteFile(out, j, 0o600)
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

type scanDriverArgs struct {
	programDir, sdkDir, cacheDir, project, stack, configFile, mainFile string
}

func runScanDriver(ctx context.Context, stderr io.Writer, tmp string, a scanDriverArgs) (*scanGraph, error) {
	script := filepath.Join(tmp, "scan.js")
	if err := os.WriteFile(script, scanDriverJS, 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(tmp, "graph.json")
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, errors.New("policy scan needs node on the PATH")
	}
	cargs := []string{
		script,
		"--program", a.programDir, "--sdk", a.sdkDir, "--cache", a.cacheDir,
		"--project", a.project, "--stack", a.stack, "--config", a.configFile, "--out", out,
	}
	if a.mainFile != "" {
		cargs = append(cargs, "--main", a.mainFile)
	}
	c := exec.CommandContext(ctx, node, cargs...)
	c.Stdout = stderr
	c.Stderr = stderr
	// The driver always writes a graph, with any errors in it; a failed run without a graph is the error.
	runErr := c.Run()
	b, err := os.ReadFile(out)
	if err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("running the program: %w", runErr)
		}
		return nil, fmt.Errorf("reading the recorded resources: %w", err)
	}
	var g scanGraph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("parsing the recorded resources: %w", err)
	}
	return &g, nil
}

// loadScanAnalyzers loads the local packs through the plugin host, as `pulumi preview --policy-pack` does. Packs of
// any runtime work. No cloud credentials are passed to them.
func loadScanAnalyzers(
	ctx context.Context, packs []engine.LocalPolicyPack,
) ([]plugin.Analyzer, func(), error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	reg := cmdCmd.NewDefaultRegistry(ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil,
		cmdutil.Diag(), env.Global())
	pluginHost, err := pkghost.New(context.WithoutCancel(ctx), cmdutil.Diag(), cmdutil.Diag(),
		nil, pkgWorkspace.EnsureLanguageInstalled,
		schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(reg))
	if err != nil {
		return nil, nil, fmt.Errorf("creating plugin host: %w", err)
	}
	pctx, err := plugin.NewContext(ctx, cmdutil.Diag(), cmdutil.Diag(), pluginHost, nil, cwd, nil, true, nil, nil)
	if err != nil {
		contract.IgnoreClose(pluginHost)
		return nil, nil, fmt.Errorf("creating plugin context: %w", err)
	}
	cleanup := func() { contract.IgnoreClose(pctx); contract.IgnoreClose(pluginHost) }
	analyzers, err := engine.LoadLocalPolicyPackAnalyzers(ctx, pctx, packs, nil)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return analyzers, cleanup, nil
}

func toURNs(ss []string) []resource.URN {
	out := make([]resource.URN, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, resource.URN(s))
		}
	}
	return out
}

// scanStackResources converts the recorded registrations into what policies see in a preview: the inputs the
// program declared, with unknowns where a value depends on something only a deployment gives, and identifiable
// unknowns where that value is another resource's output (see policy_scan_unknowns.go). It returns what stack
// policies see, and what resource policies see of each resource (its inputs only).
func scanStackResources(
	g *scanGraph, u *scanUnknowns,
) ([]plugin.AnalyzerStackResource, []plugin.AnalyzerResource, error) {
	out := make([]plugin.AnalyzerStackResource, 0, len(g.Resources))
	views := make([]plugin.AnalyzerResource, 0, len(g.Resources))
	unmarshal := func(urn string, m map[string]any) (resource.PropertyMap, error) {
		obj, err := structpb.NewStruct(m)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", urn, err)
		}
		props, err := plugin.UnmarshalProperties(obj, plugin.MarshalOptions{
			Label:         "scan." + urn,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", urn, err)
		}
		return props, nil
	}
	for i := range g.Resources {
		r := &g.Resources[i]
		inputs, _ := u.rewrite(r.Object).(map[string]any)
		if inputs == nil {
			inputs = map[string]any{}
		}
		withOutputs := make(map[string]any, len(inputs)+2)
		maps.Copy(withOutputs, inputs)
		maps.Copy(withOutputs, u.stackOutputs(r, inputs))
		props, err := unmarshal(r.URN, inputs)
		if err != nil {
			return nil, nil, err
		}
		stackProps, err := unmarshal(r.URN, withOutputs)
		if err != nil {
			return nil, nil, err
		}
		pdeps := map[resource.PropertyKey][]resource.URN{}
		for k, v := range r.PropertyDependencies {
			pdeps[resource.PropertyKey(k)] = toURNs(v)
		}
		urn := resource.URN(r.URN)
		view := plugin.AnalyzerResource{
			URN:        urn,
			Type:       urn.Type(),
			Name:       r.Name,
			Properties: resource.FromResourcePropertyMap(props),
			Options:    plugin.AnalyzerResourceOptions{Parent: resource.URN(r.Parent)},
		}
		views = append(views, view)
		view.Properties = resource.FromResourcePropertyMap(stackProps)
		out = append(out, plugin.AnalyzerStackResource{
			AnalyzerResource:     view,
			Parent:               resource.URN(r.Parent),
			Dependencies:         toURNs(r.Dependencies),
			PropertyDependencies: pdeps,
		})
	}
	return out, views, nil
}

// undecidedPrefix starts the message a policy SDK reports when a policy reads a value that's unknown before
// deployment.
const undecidedPrefix = "can't run policy"

func analyzeScanGraph(ctx context.Context, g *scanGraph, analyzers []plugin.Analyzer) (*scanReport, error) {
	unknowns := newScanUnknowns(g)
	resources, views, err := scanStackResources(g, unknowns)
	if err != nil {
		return nil, err
	}
	byURN := map[string]scanResource{}
	stackURN := ""
	for _, r := range g.Resources {
		byURN[r.URN] = r
		if r.Type == "pulumi:pulumi:Stack" {
			stackURN = r.URN
		}
	}
	root := g.CodeRoot
	if root == "" {
		root = g.ProgramDir
	}

	report := &scanReport{
		Project:    g.Project,
		ProgramDir: g.ProgramDir,
		Resources:  len(g.Resources),
		Findings:   []scanFinding{},
		Undecided:  []scanFinding{},
		Stubs:      g.Stubs,
		Missing:    g.Missing,
		Assumed:    g.Assumed,
		Blocked:    g.Blocked,
		Fixtures:   g.Fixtures,
		Unmocked:   g.Unmocked,
		Probed:     g.Probed,
		Fallbacks:  g.Fallbacks,
		Linked:     unknowns.linked,
		Unlinked:   unknowns.unlinked,
	}
	for _, e := range g.Errors {
		report.Errors = append(report.Errors, e.Message)
	}

	add := func(d plugin.AnalyzeDiagnostic, urn string) {
		if urn == "" {
			urn = stackURN
		}
		r := byURN[urn]
		f := scanFinding{
			URN:              urn,
			Type:             r.Type,
			Name:             r.Name,
			PolicyPack:       d.PolicyPackName,
			PolicyPackVer:    d.PolicyPackVersion,
			Policy:           d.PolicyName,
			Description:      d.Description,
			Severity:         string(d.Severity),
			EnforcementLevel: string(d.EnforcementLevel),
			Message:          unknowns.describe(strings.TrimSpace(d.Message)),
		}
		// Only positions in the program's own code are useful; the SDK registers the Stack, for one.
		if p := r.SourcePosition; p != nil {
			if u, err := url.Parse(p.URI); err == nil && u.Scheme == "file" {
				if rel, err := filepath.Rel(root, u.Path); err == nil && !strings.HasPrefix(rel, "..") &&
					!strings.Contains(rel, "node_modules") {
					f.URI, f.Line, f.Column, f.File = p.URI, p.Line, p.Column, rel
				}
			}
		}
		if strings.HasPrefix(f.Message, undecidedPrefix) {
			report.Undecided = append(report.Undecided, f)
		} else {
			report.Findings = append(report.Findings, f)
		}
	}

	for _, a := range analyzers {
		resp, err := a.AnalyzeStack(ctx, resources)
		if err != nil {
			return nil, err
		}
		for _, d := range resp.Diagnostics {
			add(d, string(d.URN))
		}
		// Resource policies see each resource's inputs, as in a preview.
		for _, r := range views {
			if r.Type == resource.RootStackType {
				continue
			}
			resp, err := a.Analyze(ctx, r)
			if err != nil {
				return nil, err
			}
			for _, d := range resp.Diagnostics {
				add(d, string(r.URN))
			}
		}
	}

	sortFindings := func(fs []scanFinding) {
		sort.SliceStable(fs, func(i, j int) bool {
			if fs[i].File != fs[j].File {
				return fs[i].File < fs[j].File
			}
			if fs[i].Line != fs[j].Line {
				return fs[i].Line < fs[j].Line
			}
			return fs[i].Policy < fs[j].Policy
		})
	}
	sortFindings(report.Findings)
	sortFindings(report.Undecided)
	return report, nil
}

func printScanReport(w io.Writer, r *scanReport) {
	loc := func(f scanFinding) string {
		if f.File == "" {
			return "(no source position)"
		}
		return fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)
	}
	printFindings := func(fs []scanFinding) {
		last := ""
		for _, f := range fs {
			head := fmt.Sprintf("%s  %s %s", loc(f), f.Type, f.Name)
			if head != last {
				fmt.Fprintf(w, "\n%s\n", head)
				last = head
			}
			fmt.Fprintf(w, "  %-9s %s/%s: %s\n", f.EnforcementLevel, f.PolicyPack, f.Policy,
				strings.ReplaceAll(f.Message, "\n", "\n            "))
		}
	}
	list := func(title string, items []string) {
		if len(items) > 0 {
			fmt.Fprintf(w, "\n%s: %s\n", title, strings.Join(items, ", "))
		}
	}

	fmt.Fprintf(w, "Policy scan of %s (%s): %d resources\n", r.Project, r.ProgramDir, r.Resources)
	printFindings(r.Findings)
	if len(r.Undecided) > 0 {
		fmt.Fprintf(w, "\nPolicies that can't be decided before deployment:\n")
		printFindings(r.Undecided)
	}
	list("Modules not available, replaced by stubs", r.Stubs)
	list("Side effects blocked (network, processes; results are unknown or empty)", r.Blocked)
	list("Fixtures used", r.Fixtures)
	list("Unknown without a fixture (add them to "+scanFixturesFile+" to make them known)", r.Unmocked)
	list("Build outputs not present, assumed built", r.Assumed)
	list("Config not set, placeholders used", r.Missing)
	if r.Probed > 0 || r.Fallbacks > 0 {
		fmt.Fprintf(w, "\napply callbacks run on unknown values with probes: %d (%d fell back to unknown)\n",
			r.Probed, r.Fallbacks)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(w, "\nerror: %s\n", e)
	}
	mandatory, advisory := 0, 0
	for _, f := range r.Findings {
		if f.EnforcementLevel == string(apitype.Mandatory) {
			mandatory++
		} else {
			advisory++
		}
	}
	fmt.Fprintf(w, "\n%d mandatory, %d advisory violations; %d policy results can't be decided before deployment "+
		"(%dms)\n", mandatory, advisory, len(r.Undecided), r.Timings["totalMs"])
}

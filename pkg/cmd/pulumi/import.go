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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/importer"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
)

func parseResourceSpec(spec string) (string, resource.URN, error) {
	equals := strings.Index(spec, "=")
	if equals == -1 {
		return "", "", errors.New("spec must be of the form name=URN")
	}

	name, urn := spec[:equals], resource.URN(spec[equals+1:])
	if name == "" || urn == "" {
		return "", "", errors.New("spec must be of the form name=URN")
	}

	if !urn.IsValid() {
		if ref, err := providers.ParseReference(string(urn)); err == nil {
			return "", "", fmt.Errorf("expected a URN but got a Provider Reference, use '%s' instead", ref.URN())
		}
		return "", "", fmt.Errorf("expected a URN but got '%s'", urn)
	}

	return name, urn, nil
}

func makeImportFileFromResourceList(resources []plugin.ResourceImport) (importFile, error) {
	nameTable := map[string]resource.URN{}
	specs := make([]importSpec, len(resources))
	for i, res := range resources {
		specs[i] = importSpec{
			Type:              tokens.Type(res.Type),
			Name:              res.Name,
			ID:                resource.ID(res.ID),
			Version:           res.Version,
			PluginDownloadURL: res.PluginDownloadURL,
			// TODO(https://github.com/pulumi/pulumi/issues/14532): Add Component, Remote, and LogicalName to ResourceImport
		}
	}

	return importFile{
		NameTable: nameTable,
		Resources: specs,
	}, nil
}

func makeImportFile(
	typ, name, id string,
	properties []string,
	parentSpec, providerSpec, version string,
) (importFile, error) {
	nameTable := map[string]resource.URN{}
	res := importSpec{
		Type:       tokens.Type(typ),
		Name:       name,
		ID:         resource.ID(id),
		Version:    version,
		Properties: properties,
	}

	if parentSpec != "" {
		parentName, parentURN, err := parseResourceSpec(parentSpec)
		if err != nil {
			parentName = "parent"
			parentURN = resource.URN(parentSpec)
			if !parentURN.IsValid() {
				return importFile{}, fmt.Errorf("invalid parent URN: '%s'", parentURN)
			}
		}
		nameTable[parentName] = parentURN
		res.Parent = parentName
	}

	if providerSpec != "" {
		providerName, providerURN, err := parseResourceSpec(providerSpec)
		if err != nil {
			providerName = "provider"
			providerURN = resource.URN(providerSpec)
		}
		if _, exists := nameTable[providerName]; exists {
			return importFile{}, fmt.Errorf("provider and parent must have distinct names, both were '%s'", providerName)
		}
		nameTable[providerName] = providerURN
		res.Provider = providerName
	}

	return importFile{
		NameTable: nameTable,
		Resources: []importSpec{res},
	}, nil
}

type importSpec struct {
	Type              tokens.Type `json:"type"`
	Name              string      `json:"name"`
	ID                resource.ID `json:"id,omitempty"`
	Parent            string      `json:"parent,omitempty"`
	Provider          string      `json:"provider,omitempty"`
	Version           string      `json:"version,omitempty"`
	PluginDownloadURL string      `json:"pluginDownloadUrl,omitempty"`
	Properties        []string    `json:"properties,omitempty"`
	Component         bool        `json:"component,omitempty"`
	Remote            bool        `json:"remote,omitempty"`

	// LogicalName is the resources Pulumi name (i.e. the first argument to `new Resource`).
	LogicalName string `json:"logicalName,omitempty"`
}

type importFile struct {
	NameTable map[string]resource.URN `json:"nameTable,omitempty"`
	Resources []importSpec            `json:"resources"`
}

func readImportFile(p string) (importFile, error) {
	f, err := os.Open(p)
	if err != nil {
		return importFile{}, err
	}
	defer contract.IgnoreClose(f)

	var result importFile
	if err = json.NewDecoder(f).Decode(&result); err != nil {
		return importFile{}, err
	}
	return result, nil
}

func writeImportFile(v importFile, f io.Writer) error {
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	err := enc.Encode(v)
	return err
}

func writeImportFileToTemp(v importFile) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	f, err := os.CreateTemp(wd, "pulumi-import-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()
	defer contract.IgnoreClose(f)

	err = writeImportFile(v, f)
	if err != nil {
		return "", errors.Join(err, f.Close(), os.Remove(path))
	}
	return path, f.Close()
}

func parseImportFile(
	f importFile, stack tokens.StackName, proj tokens.PackageName, protectResources bool,
) ([]deploy.Import, importer.NameTable, error) {
	// First check for uniqueness and ambiguity, takenNames tracks both that a name is used (it's in the map) and if
	// it's ambiguous (it's true).
	takenNames := map[string]bool{}
	// takenURNs simply tracks that a URN is used, it's not possible for a URN to be ambiguous. We fill this
	// later because it requires parents to be resolved.
	takenURNs := map[resource.URN]struct{}{}
	// Prefill takenNames with all the resource names so we can do quick uniqueness checks below
	for _, spec := range f.Resources {
		takenNames[spec.Name] = false
	}

	// TODO: When Go 1.21 is released, switch to errors.Join.
	var errs error
	pusherrf := func(format string, args ...interface{}) {
		errs = multierror.Append(errs, fmt.Errorf(format, args...))
	}

	// Attempts to generate a human-readable description of the given import spec
	// for use in error messages using whatever information is available.
	// For example:
	//
	//	resource 'foo' of type 'aws:ec2/vpc:Vpc'
	//	resource 'foo'
	//	resource 3 of type 'aws:ec2/vpc:Vpc'
	//	resource 3
	describeResource := func(idx int, spec importSpec) string {
		var sb strings.Builder
		sb.WriteString("resource ")

		switch {
		case spec.Name != "":
			fmt.Fprintf(&sb, "'%v'", spec.Name)
		case spec.ID != "":
			fmt.Fprintf(&sb, "'%v'", spec.ID)
		default:
			fmt.Fprintf(&sb, "%d", idx)
		}

		if spec.Type != "" {
			fmt.Fprintf(&sb, " of type '%v'", spec.Type)
		}

		return sb.String()
	}

	for i, spec := range f.Resources {
		// We default LogicalName and Name to each other if either is missing.
		if spec.LogicalName == "" {
			f.Resources[i].LogicalName = spec.Name
		}
		if spec.Name == "" {
			f.Resources[i].Name = spec.LogicalName
		}
	}

	// Sanity check some basic constraints that names and types etc are filled in.
	for i, spec := range f.Resources {
		if spec.Type == "" {
			pusherrf("%v has no type", describeResource(i, spec))
		}
		if spec.Name == "" {
			pusherrf("%v has no name", describeResource(i, spec))
		}
		if !spec.Component && spec.ID == "" {
			pusherrf("%v has no ID", describeResource(i, spec))
		} else if spec.Component && spec.ID != "" {
			pusherrf("%v has an ID, but is marked as a component", describeResource(i, spec))
		}
		if spec.Remote && !spec.Component {
			pusherrf("%v is marked as remote, but not as a component", describeResource(i, spec))
		}

		// Check if any earlier resource has this name, if so mark it as ambiguous this is only an error if
		// something tries to use it (checked later on).
		for j, other := range f.Resources {
			if i > j && spec.Name == other.Name {
				takenNames[spec.Name] = true
			}
		}
	}

	// If we've got errors already just exit
	if errs != nil {
		return nil, nil, errs
	}

	// A mapping from name to URN, prefilled with emptys and what was in the name table so we can do existence checks
	// for expected names.
	urnMapping := make(map[string]resource.URN)
	// The opposite of urnMapping, a mapping from URN to name.
	names := importer.NameTable{}
	for name, urn := range f.NameTable {
		names[urn] = name
		urnMapping[name] = urn
		// We can add these URNs to the taken set, it's not an error to add them twice at this point.
		takenURNs[urn] = struct{}{}
	}
	for _, spec := range f.Resources {
		urnMapping[spec.Name] = ""
	}

	// We need to keep going till all the resources are resolved.
	dones := make([]bool, len(f.Resources))
	done := func() bool {
		if errs != nil {
			return true
		}
		for _, done := range dones {
			if !done {
				return false
			}
		}
		return true
	}

	for !done() {
		for i, spec := range f.Resources {
			// If we've already done this URN no need to do it again
			if dones[i] {
				continue
			}

			var parentType tokens.Type
			if spec.Parent != "" {
				// We can find the parent type by looking up the parent by name then finding it's type

				// takenNames will be true if this name is ambiguous, in which case we can't use it as a
				// parent but we just let the rest of the code below run so we can collect further errors.
				if takenNames[spec.Parent] {
					pusherrf("%v has an ambiguous parent",
						describeResource(i, spec))
				}

				// Is this name already in the name table?
				if urn, ok := f.NameTable[spec.Parent]; ok {
					parentType = urn.QualifiedType()
				} else {
					// Not in the name table, is it in the urn mapping yet?
					urn, ok := urnMapping[spec.Parent]

					// There's three cases to cover here:
					// 1. We didn't find the parent, in which case just push an error
					// 2. We found the parent but it's urn is currently blank, in which case we'll loop around
					// 3. We found the parent and got it's URN
					if !ok {
						pusherrf("the parent '%v' for %v has no entry in 'nameTable'",
							spec.Parent, describeResource(i, spec))
					} else if urn == "" {
						// Skip this resource for now, we'll have to loop again to get it once it's parent URN is worked out
						continue
					} else {
						parentType = urn.QualifiedType()
					}
				}
			}

			urn := resource.NewURN(stack.Q(), proj, parentType, spec.Type, spec.LogicalName)
			// Check if this URN is unique and if not add an error
			if _, ok := takenURNs[urn]; ok {
				pusherrf("%v has an ambiguous URN, set name (or logical name) to be unique", describeResource(i, spec))
			}
			urnMapping[spec.Name] = urn
			takenURNs[urn] = struct{}{}
			// This might clash with earlier entries in the name table (unique urn, but duplicate name) that's
			// fine and the code generators should deal with it.
			names[urn] = spec.Name
			// Mark this resource as done
			dones[i] = true
		}
	}

	// If we've got errors already just exit
	if errs != nil {
		return nil, nil, errs
	}

	imports := make([]deploy.Import, len(f.Resources))
	for i, spec := range f.Resources {
		imp := deploy.Import{
			Type:              spec.Type,
			Name:              spec.LogicalName,
			ID:                spec.ID,
			Protect:           protectResources,
			Properties:        spec.Properties,
			PluginDownloadURL: spec.PluginDownloadURL,
			Component:         spec.Component,
			Remote:            spec.Remote,
		}

		if spec.Parent != "" {
			urn, ok := urnMapping[spec.Parent]
			if ok {
				// No need to add errors here, we'll have done that above when building URNs
				imp.Parent = urn
			}
		}

		if spec.Provider != "" {
			if takenNames[spec.Provider] {
				pusherrf("%v has an ambiguous provider",
					describeResource(i, spec))
			}
			urn, ok := f.NameTable[spec.Provider]
			if !ok {
				pusherrf("the provider '%v' for %v has no entry in 'nameTable'",
					spec.Provider, describeResource(i, spec))
			} else {
				imp.Provider = urn
			}
		}

		if spec.Version != "" {
			v, err := semver.ParseTolerant(spec.Version)
			if err != nil {
				pusherrf("could not parse version '%v' for %v: %w",
					spec.Version, describeResource(i, spec), err)
			} else {
				imp.Version = &v
			}
		}

		imports[i] = imp
	}

	return imports, names, errs
}

func getCurrentDeploymentForStack(
	ctx context.Context,
	s backend.Stack,
) (*deploy.Snapshot, error) {
	deployment, err := s.ExportDeployment(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := stack.DeserializeUntypedDeployment(ctx, deployment, stack.DefaultSecretsProvider)
	if err != nil {
		switch err {
		case stack.ErrDeploymentSchemaVersionTooOld:
			return nil, fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
				s.Ref().Name())
		case stack.ErrDeploymentSchemaVersionTooNew:
			return nil, fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
				"Please update your version of the Pulumi CLI", s.Ref().Name())
		}
		return nil, fmt.Errorf("could not deserialize deployment: %w", err)
	}
	return snap, err
}

type programGeneratorFunc func(
	p *pcl.Program,
	loader schema.ReferenceLoader,
) (map[string][]byte, hcl.Diagnostics, error)

func generateImportedDefinitions(ctx *plugin.Context,
	out io.Writer, stackName tokens.StackName, projectName tokens.PackageName,
	snap *deploy.Snapshot, programGenerator programGeneratorFunc, names importer.NameTable,
	imports []deploy.Import, protectResources bool,
) (bool, error) {
	defer func() {
		v := recover()
		if v != nil {
			errMsg := strings.Builder{}
			errMsg.WriteString("Your resource has been imported into Pulumi state, but there was an error generating the import code.\n") //nolint:lll
			errMsg.WriteString("\n")
			if strings.Contains(fmt.Sprintf("%v", v), "invalid Go source code:") {
				errMsg.WriteString("You will need to copy and paste the generated code into your Pulumi application and manually edit it to correct any errors.\n\n") //nolint:lll
			}
			fmt.Fprintf(&errMsg, "%v\n", v)
			fmt.Print(errMsg.String())
		}
	}()

	resourceTable := map[resource.URN]*resource.State{}
	for _, r := range snap.Resources {
		if !r.Delete {
			resourceTable[r.URN] = r
		}
	}

	var resources []*resource.State
	for _, i := range imports {
		var parentType tokens.Type
		if i.Parent != "" {
			parentType = i.Parent.QualifiedType()
		}
		urn := resource.NewURN(stackName.Q(), projectName, parentType, i.Type, i.Name)
		if state, ok := resourceTable[urn]; ok {
			// Copy the state and override the protect bit.
			s := *state
			s.Protect = protectResources
			resources = append(resources, &s)
		}
	}

	if len(resources) == 0 {
		return false, nil
	}

	loader := schema.NewPluginLoader(ctx.Host)
	return true, importer.GenerateLanguageDefinitions(out, loader, func(w io.Writer, p *pcl.Program) error {
		files, _, err := programGenerator(p, loader)
		if err != nil {
			return err
		}

		var contents []byte
		for _, v := range files {
			contents = v
		}

		if _, err := w.Write(contents); err != nil {
			return err
		}
		return nil
	}, resources, names)
}

func newImportCmd() *cobra.Command {
	var parentSpec string
	var providerSpec string
	var importFilePath string
	var outputFilePath string
	var generateCode bool

	var debug bool
	var message string
	var stackName string
	var execKind string
	var execAgent string

	// Flags for engine.UpdateOptions.
	var diffDisplay bool
	var eventLogPath string
	var parallel int
	var showConfig bool
	var skipPreview bool
	var suppressOutputs bool
	var suppressPermalink string
	var yes bool
	var protectResources bool
	var properties []string

	var from string

	cmd := &cobra.Command{
		Use:   "import [type] [name] [id]",
		Short: "Import resources into an existing stack",
		Long: "Import resources into an existing stack.\n" +
			"\n" +
			"Resources that are not managed by Pulumi can be imported into a Pulumi stack\n" +
			"using this command. A definition for each resource will be printed to stdout\n" +
			"in the language used by the project associated with the stack; these definitions\n" +
			"should be added to the Pulumi program. The resources are protected from deletion\n" +
			"by default.\n" +
			"\n" +
			"Should you want to import your resource(s) without protection, you can pass\n" +
			"`--protect=false` as an argument to the command. This will leave all resources unprotected.\n" +
			"\n" +
			"A single resource may be specified in the command line arguments or a set of\n" +
			"resources may be specified by a JSON file.\n" +
			"\n" +
			"If using the command line args directly, the type, name, id and optional flags\n" +
			"must be provided.  For example:\n" +
			"\n" +
			"    pulumi import 'aws:iam/user:User' name id\n" +
			"\n" +
			"Or to fully specify parent and/or provider, subsitute the <urn> for each into the following:\n" +
			"\n" +
			"     pulumi import 'aws:iam/user:User' name id --parent 'parent=<urn>' --provider 'admin=<urn>'\n" +
			"\n" +
			"If using the JSON file format to define the imported resource(s), use this instead:\n" +
			"\n" +
			"     pulumi import -f import.json\n" +
			"\n" +
			"Where import.json is a file that matches the following JSON format:\n" +
			"\n" +
			"    {\n" +
			"        \"nameTable\": {\n" +
			"            \"provider-or-parent-name-0\": \"provider-or-parent-urn-0\",\n" +
			"            ...\n" +
			"            \"provider-or-parent-name-n\": \"provider-or-parent-urn-n\",\n" +
			"        },\n" +
			"        \"resources\": [\n" +
			"            {\n" +
			"                \"type\": \"type-token\",\n" +
			"                \"name\": \"name\",\n" +
			"                \"id\": \"resource-id\",\n" +
			"                \"parent\": \"optional-parent-name\",\n" +
			"                \"provider\": \"optional-provider-name\",\n" +
			"                \"version\": \"optional-provider-version\",\n" +
			"                \"properties\": [\"optional-property-names\"],\n" +
			"            },\n" +
			"            ...\n" +
			"            {\n" +
			"                ...\n" +
			"            }\n" +
			"        ]\n" +
			"    }\n" +
			"\n" +
			"The name table maps language names to parent and provider URNs. These names are\n" +
			"used in the generated definitions, and should match the corresponding declarations\n" +
			"in the source program. This table is required if any parents or providers are\n" +
			"specified by the resources to import.\n" +
			"\n" +
			"The resources list contains the set of resources to import. Each resource is\n" +
			"specified as a triple of its type, name, and ID. The format of the ID is specific\n" +
			"to the resource type. Each resource may specify the name of a parent or provider;\n" +
			"these names must correspond to entries in the name table. If a resource does not\n" +
			"specify a provider, it will be imported using the default provider for its type. A\n" +
			"resource that does specify a provider may specify the version of the provider\n" +
			"that will be used for its import.\n" +
			"\n" +
			"Each resource may specify which input properties to import with;\n" +
			"\n" +
			"If a resource does not specify any properties the default behaviour is to\n" +
			"import using all required properties.\n" +
			"\n" +
			"You can use `pulumi preview` with the `--import-file` option to emit an import file\n" +
			"for all resources that need creating from the preview. This will fill in all the name,\n" +
			"type, parent and provider information for you and just require you to fill in resource\n" +
			"IDs and any properties.\n",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := commandContext()

			cwd, err := os.Getwd()
			if err != nil {
				return result.FromError(fmt.Errorf("get working directory: %w", err))
			}
			sink := cmdutil.Diag()
			pCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("create plugin context: %w", err))
			}

			var importFile importFile
			if importFilePath != "" {
				if len(args) != 0 || parentSpec != "" || providerSpec != "" || len(properties) != 0 {
					contract.IgnoreError(cmd.Help())
					return result.Errorf("an inline resource may not be specified in conjunction with an import file")
				}
				if from != "" {
					contract.IgnoreError(cmd.Help())
					return result.Errorf("a converter may not be specified in conjunction with an import file")
				}
				f, err := readImportFile(importFilePath)
				if err != nil {
					return result.FromError(fmt.Errorf("could not read import file: %w", err))
				}
				importFile = f
			} else if from != "" {
				log := func(sev diag.Severity, msg string) {
					pCtx.Diag.Logf(sev, diag.RawMessage("", msg))
				}
				converter, err := loadConverterPlugin(pCtx, from, log)
				if err != nil {
					return result.Errorf("load converter plugin: %w", err)
				}
				defer contract.IgnoreClose(converter)

				installProvider := func(provider tokens.Package) *semver.Version {
					// If auto plugin installs are disabled just return nil, the mapper will still carry on
					if env.DisableAutomaticPluginAcquisition.Value() {
						return nil
					}

					log := func(sev diag.Severity, msg string) {
						pCtx.Diag.Logf(sev, diag.RawMessage("", msg))
					}

					pluginSpec := workspace.PluginSpec{
						Name: string(provider),
						Kind: workspace.ResourcePlugin,
					}
					version, err := pkgWorkspace.InstallPlugin(pluginSpec, log)
					if err != nil {
						pCtx.Diag.Warningf(diag.Message("", "failed to install provider %q: %v"), provider, err)
						return nil
					}
					return version
				}

				mapper, err := convert.NewPluginMapper(
					convert.DefaultWorkspace(), convert.ProviderFactoryFromHost(pCtx.Host),
					from, nil, installProvider)
				if err != nil {
					return result.FromError(err)
				}

				mapperServer := convert.NewMapperServer(mapper)
				grpcServer, err := plugin.NewServer(pCtx, convert.MapperRegistration(mapperServer))
				if err != nil {
					return result.FromError(err)
				}

				resp, err := converter.ConvertState(ctx, &plugin.ConvertStateRequest{
					MapperTarget: grpcServer.Addr(),
					Args:         args,
				})
				if err != nil {
					return result.FromError(err)
				}

				printDiagnostics(sink, resp.Diagnostics)
				if resp.Diagnostics.HasErrors() {
					// If we've got error diagnostics then state conversion failed, we've printed the error above so
					// just return a plain message here.
					return result.Error("conversion failed")
				}

				f, err := makeImportFileFromResourceList(resp.Resources)
				if err != nil {
					return result.FromError(err)
				}
				importFile = f
			} else {
				msg := "an inline resource must be specified if no converter or import file is used, missing "
				if len(args) == 0 {
					contract.IgnoreError(cmd.Help())
					return result.Errorf(msg + "type, name, and id")
				}
				if len(args) == 1 {
					contract.IgnoreError(cmd.Help())
					return result.Errorf(msg + "name and id")
				}
				if len(args) == 2 {
					contract.IgnoreError(cmd.Help())
					return result.Errorf(msg + "id")
				}
				if len(args) > 3 {
					contract.IgnoreError(cmd.Help())
					return result.Errorf("only expected at most three arguments")
				}
				f, err := makeImportFile(args[0], args[1], args[2], properties, parentSpec, providerSpec, "")
				if err != nil {
					return result.FromError(err)
				}
				importFile = f
			}

			if !generateCode && outputFilePath != "" {
				fmt.Fprintln(os.Stderr, "Output file will not be used as --generate-code is false.")
			}

			var outputResult bytes.Buffer
			output := io.Writer(&outputResult)
			if outputFilePath != "" {
				f, err := os.Create(outputFilePath)
				if err != nil {
					return result.Errorf("could not open output file: %v", err)
				}
				defer contract.IgnoreClose(f)
				output = f
			}

			// Fetch the project.
			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			yes = yes || skipPreview || skipConfirmations()
			interactive := cmdutil.Interactive()
			if !interactive && !yes {
				return result.FromError(
					errors.New("--yes or --skip-preview must be passed in to proceed when running in non-interactive mode"))
			}

			opts, err := updateFlagsToOptions(interactive, skipPreview, yes)
			if err != nil {
				return result.FromError(err)
			}

			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			opts.Display = display.Options{
				Color:           cmdutil.GetGlobalColorization(),
				ShowConfig:      showConfig,
				SuppressOutputs: suppressOutputs,
				IsInteractive:   interactive,
				Type:            displayType,
				EventLogPath:    eventLogPath,
				Debug:           debug,
			}

			// we only suppress permalinks if the user passes true. the default is an empty string
			// which we pass as 'false'
			if suppressPermalink == "true" {
				opts.Display.SuppressPermalink = true
			} else {
				opts.Display.SuppressPermalink = false
			}

			filestateBackend, err := isFilestateBackend(opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			// by default, we are going to suppress the permalink when using self-managed backends
			// this can be re-enabled by explicitly passing "false" to the `suppress-permalink` flag
			if suppressPermalink != "false" && filestateBackend {
				opts.Display.SuppressPermalink = true
			}

			// Fetch the current stack.
			s, err := requireStack(ctx, stackName, stackLoadOnly, opts.Display)
			if err != nil {
				return result.FromError(err)
			}

			imports, nameTable, err := parseImportFile(importFile, s.Ref().Name(), proj.Name, protectResources)
			if err != nil {
				return result.FromError(err)
			}

			wrapper := func(
				f func(*pcl.Program) (map[string][]byte, hcl.Diagnostics, error),
			) func(*pcl.Program, schema.ReferenceLoader) (map[string][]byte, hcl.Diagnostics, error) {
				return func(p *pcl.Program, loader schema.ReferenceLoader) (map[string][]byte, hcl.Diagnostics, error) {
					return f(p)
				}
			}

			var programGenerator programGeneratorFunc
			switch proj.Runtime.Name() {
			case "dotnet":
				programGenerator = wrapper(dotnet.GenerateProgram)
			case "java":
				programGenerator = wrapper(javagen.GenerateProgram)
			case "yaml":
				programGenerator = wrapper(yamlgen.GenerateProgram)
			default:
				programGenerator = func(
					program *pcl.Program, loader schema.ReferenceLoader,
				) (map[string][]byte, hcl.Diagnostics, error) {
					cwd, err := os.Getwd()
					if err != nil {
						return nil, nil, err
					}
					sink := cmdutil.Diag()

					ctx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
					if err != nil {
						return nil, nil, err
					}
					defer contract.IgnoreClose(pCtx.Host)

					languagePlugin, err := ctx.Host.LanguageRuntime(cwd, cwd, proj.Runtime.Name(), nil)
					if err != nil {
						return nil, nil, err
					}

					loaderServer := schema.NewLoaderServer(loader)
					grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
					if err != nil {
						return nil, nil, err
					}
					defer contract.IgnoreClose(grpcServer)

					files, diagnostics, err := languagePlugin.GenerateProgram(program.Source(), grpcServer.Addr())
					if err != nil {
						return nil, nil, err
					}

					return files, diagnostics, nil
				}
			}

			m, err := getUpdateMetadata(message, root, execKind, execAgent, false, cmd.Flags())
			if err != nil {
				return result.FromError(fmt.Errorf("gathering environment metadata: %w", err))
			}

			cfg, sm, err := getStackConfiguration(ctx, s, proj, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack configuration: %w", err))
			}

			decrypter, err := sm.Decrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack decrypter: %w", err))
			}
			encrypter, err := sm.Encrypter()
			if err != nil {
				return result.FromError(fmt.Errorf("getting stack encrypter: %w", err))
			}

			stackName := s.Ref().Name().String()
			configErr := workspace.ValidateStackConfigAndApplyProjectConfig(
				stackName,
				proj,
				cfg.Environment,
				cfg.Config,
				encrypter,
				decrypter)
			if configErr != nil {
				return result.FromError(fmt.Errorf("validating stack config: %w", configErr))
			}

			opts.Engine = engine.UpdateOptions{
				Parallel:      parallel,
				Debug:         debug,
				UseLegacyDiff: useLegacyDiff(),
				Experimental:  hasExperimentalCommands(),
			}

			_, res := s.Import(ctx, backend.UpdateOperation{
				Proj:               proj,
				Root:               root,
				M:                  m,
				Opts:               opts,
				StackConfiguration: cfg,
				SecretsManager:     sm,
				SecretsProvider:    stack.DefaultSecretsProvider,
				Scopes:             backend.CancellationScopes,
			}, imports)

			if generateCode {
				deployment, err := getCurrentDeploymentForStack(ctx, s)
				if err != nil {
					return result.FromError(err)
				}

				validImports, err := generateImportedDefinitions(
					pCtx, output, s.Ref().Name(), proj.Name, deployment, programGenerator, nameTable, imports,
					protectResources)
				if err != nil {
					if _, ok := err.(*importer.DiagnosticsError); ok {
						err = fmt.Errorf("internal error: %w", err)
					}
					return result.FromError(err)
				}

				if validImports {
					// we only want to output the helper string if there is a set of valid imports to convert into code
					// this protects against invalid package types or import errors that will not actually result in
					// in a codegen call
					// It's a little bit more memory but is a better experience that writing to stdout and then an error
					// occurring
					if outputFilePath == "" {
						fmt.Print("Please copy the following code into your Pulumi application. Not doing so\n" +
							"will cause Pulumi to report that an update will happen on the next update command.\n\n")
						if protectResources {
							fmt.Print(("Please note that the imported resources are marked as protected. " +
								"To destroy them\n" +
								"you will need to remove the `protect` option and run `pulumi update` *before*\n" +
								"the destroy will take effect.\n\n"))
						}
						fmt.Print(outputResult.String())
					}
				}
			}

			if res != nil {
				if res.Error() == context.Canceled {
					return result.FromError(errors.New("import cancelled"))
				}

				// If we did a conversion import (i.e. from!="") then lets write the file we've built out to the local
				// directory so if there's any issues users can manually edit the file and try again with --file
				if from != "" {
					path, err := writeImportFileToTemp(importFile)
					if err != nil {
						return result.FromError(err)
					}
					pCtx.Diag.Infof(diag.Message("",
						"Generated import file written out, edit and rerun import with --file %s"),
						path, path)
				}

				return PrintEngineResult(res)
			}
			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		//nolint:lll
		&parentSpec, "parent", "", "The name and URN of the parent resource in the format name=urn, where name is the variable name of the parent resource")
	cmd.PersistentFlags().StringVar(
		//nolint:lll
		&providerSpec, "provider", "", "The name and URN of the provider to use for the import in the format name=urn, where name is the variable name for the provider resource")
	cmd.PersistentFlags().StringSliceVar(
		//nolint:lll
		&properties, "properties", nil, "The property names to use for the import in the format name1,name2")
	cmd.PersistentFlags().StringVarP(
		&importFilePath, "file", "f", "", "The path to a JSON-encoded file containing a list of resources to import")
	cmd.PersistentFlags().StringVarP(
		&outputFilePath, "out", "o", "", "The path to the file that will contain the generated resource declarations")
	cmd.PersistentFlags().BoolVar(
		&generateCode, "generate-code", true, "Generate resource declaration code for the imported resources")

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", defaultParallel,
		"Allow P resource operations to run in parallel at once (1 for no parallelism).")
	cmd.PersistentFlags().BoolVar(
		&skipPreview, "skip-preview", false,
		"Do not calculate a preview before performing the import")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")
	cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically approve and perform the import after previewing it")
	cmd.PersistentFlags().BoolVarP(
		&protectResources, "protect", "", true,
		"Allow resources to be imported with protection from deletion enabled")
	cmd.PersistentFlags().StringVar(
		&from, "from", "",
		"Invoke a converter to import the resources")

	if hasDebugCommands() {
		cmd.PersistentFlags().StringVar(
			&eventLogPath, "event-log", "",
			"Log events to a file at this path")
	}

	// internal flags
	cmd.PersistentFlags().StringVar(&execKind, "exec-kind", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-kind")
	cmd.PersistentFlags().StringVar(&execAgent, "exec-agent", "", "")
	// ignore err, only happens if flag does not exist
	_ = cmd.PersistentFlags().MarkHidden("exec-agent")

	return cmd
}

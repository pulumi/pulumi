// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"sort"
	"strconv"

	goerr "github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/eval/heapstate"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/graph/dotconv"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newPlanCmd() *cobra.Command {
	var analyzers []string
	var dotOutput bool
	var env string
	var showConfig bool
	var showReplaceSteps bool
	var showUnchanged bool
	var summary bool
	var output string
	var cmd = &cobra.Command{
		Use:     "plan [<package>] [-- [<args>]]",
		Aliases: []string{"dryrun"},
		Short:   "Show a plan to update, create, and delete an environment's resources",
		Long: "Show a plan to update, create, and delete an environment's resources\n" +
			"\n" +
			"This command displays a plan to update an existing environment whose state is represented by\n" +
			"an existing snapshot file.  The new desired state is computed by compiling and evaluating an\n" +
			"executable package, and extracting all resource allocations from its resulting object graph.\n" +
			"This graph is compared against the existing state to determine what operations must take\n" +
			"place to achieve the desired state.  No changes to the environment will actually take place.\n" +
			"\n" +
			"By default, the package to execute is loaded from the current directory.  Optionally, an\n" +
			"explicit path can be provided using the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmdName(tokens.QName(env), args)
			if err != nil {
				return err
			}
			defer info.Close()
			deploy(cmd, info, deployOptions{
				Delete:           false,
				DryRun:           true,
				Analyzers:        analyzers,
				ShowConfig:       showConfig,
				ShowReplaceSteps: showReplaceSteps,
				ShowUnchanged:    showUnchanged,
				Summary:          summary,
				DOT:              dotOutput,
				Output:           output,
			})
			return nil
		}),
	}

	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this deployment")
	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the plan as a DOT digraph (graph description language)")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplaceSteps, "show-replace-steps", false,
		"Show detailed resource replacement creates and deletes; normally shows as a single step")
	cmd.PersistentFlags().BoolVar(
		&showUnchanged, "show-unchanged", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().StringVarP(
		&output, "output", "o", "",
		"Serialize the resulting plan to a file instead of simply printing it")

	return cmd
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, info *envCmdInfo, opts deployOptions) *planResult {
	// If deleting, there is no need to create a new snapshot; otherwise, we will need to compile the package.
	var new resource.Snapshot
	var result *compileResult
	var analyzers []tokens.QName
	if !opts.Delete {
		// First, compile; if that yields errors or an empty heap, exit early.
		if result = compile(cmd, info.Args, info.Env.Config); result == nil || result.Heap == nil {
			return nil
		}

		// Next, if a DOT output is requested, generate it and quite right now.
		// TODO: generate this DOT from the snapshot/diff, not the raw object graph.
		if opts.DOT {
			// Convert the output to a DOT file.
			if err := dotconv.Print(result.Heap.G, os.Stdout); err != nil {
				cmdutil.Sink().Errorf(errors.ErrorIO,
					goerr.Errorf("failed to write DOT file to output: %v", err))
			}
			return nil
		}

		// Create a resource snapshot from the compiled/evaluated object graph.
		var err error
		new, err = resource.NewGraphSnapshot(
			info.Ctx, info.Env.Name, result.Pkg.Tok, result.C.Ctx().Opts.Args, result.Heap, info.Old)
		if err != nil {
			result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
			return nil
		} else if !info.Ctx.Diag.Success() {
			return nil
		}

		// If there are any analyzers to run, queue them up.
		for _, a := range opts.Analyzers {
			analyzers = append(analyzers, tokens.QName(a)) // from the command line.
		}
		if as := result.Pkg.Node.Analyzers; as != nil {
			for _, a := range *as {
				analyzers = append(analyzers, a) // from the project file.
			}
		}
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan, err := resource.NewPlan(info.Ctx, info.Old, new, analyzers)
	if err != nil {
		result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
		return nil
	}
	if !info.Ctx.Diag.Success() {
		return nil
	}
	return &planResult{
		compileResult: result,
		Info:          info,
		New:           new,
		Plan:          plan,
	}
}

type planResult struct {
	*compileResult
	Info *envCmdInfo       // plan command information.
	Old  resource.Snapshot // the existing snapshot (if any).
	New  resource.Snapshot // the new snapshot for this plan (if any).
	Plan resource.Plan     // the plan created by this command.
}

func checkEmpty(d diag.Sink, plan resource.Plan) bool {
	// If we are doing an empty update, say so.
	if plan.Empty() {
		d.Infof(diag.Message("no resources need to be updated"))
		return true
	}
	return false
}

func prepareCompiler(cmd *cobra.Command, args []string) (compiler.Compiler, *pack.Package) {
	// If there's a --, we need to separate out the command args from the stack args.
	flags := cmd.Flags()
	dashdash := flags.ArgsLenAtDash()
	var packArgs []string
	if dashdash != -1 {
		packArgs = args[dashdash:]
		args = args[0:dashdash]
	}

	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()
	opts.Args = dashdashArgsToMap(packArgs)

	// In the case of an argument, load that specific package and new up a compiler based on its base path.
	// Otherwise, use the default workspace and package logic (which consults the current working directory).
	var comp compiler.Compiler
	var pkg *pack.Package
	if len(args) == 0 {
		var err error
		comp, err = compiler.Newwd(opts)
		if err != nil {
			// Create a temporary diagnostics sink so that we can issue an error and bail out.
			cmdutil.Sink().Errorf(errors.ErrorCantCreateCompiler, err)
		}
	} else {
		fn := args[0]
		if pkg = cmdutil.ReadPackageFromArg(fn); pkg != nil {
			var err error
			if fn == "-" {
				comp, err = compiler.Newwd(opts)
			} else {
				comp, err = compiler.New(filepath.Dir(fn), opts)
			}
			if err != nil {
				cmdutil.Sink().Errorf(errors.ErrorCantReadPackage, fn, err)
			}
		}
	}

	return comp, pkg
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// LumiGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
func compile(cmd *cobra.Command, args []string, config resource.ConfigMap) *compileResult {
	// Prepare the compiler info and, provided it succeeds, perform the compilation.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		// Create the preexec hook if the config map is non-nil.
		var preexec compiler.Preexec
		configVars := make(map[tokens.Token]*rt.Object)
		if config != nil {
			preexec = config.ConfigApplier(configVars)
		}

		// Now perform the compilation and extract the heap snapshot.
		var heap *heapstate.Heap
		var pkgsym *symbols.Package
		if pkg == nil {
			pkgsym, heap = comp.Compile(preexec)
		} else {
			pkgsym, heap = comp.CompilePackage(pkg, preexec)
		}

		return &compileResult{
			C:          comp,
			Pkg:        pkgsym,
			Heap:       heap,
			ConfigVars: configVars,
		}
	}

	return nil
}

type compileResult struct {
	C          compiler.Compiler
	Pkg        *symbols.Package
	Heap       *heapstate.Heap
	ConfigVars map[tokens.Token]*rt.Object
}

// verify creates a compiler, much like compile, but only performs binding and verification on it.  If verification
// succeeds, the return value is true; if verification fails, errors will have been output, and the return is false.
func verify(cmd *cobra.Command, args []string) bool {
	// Prepare the compiler info and, provided it succeeds, perform the verification.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		// Now perform the compilation and extract the heap snapshot.
		if pkg == nil {
			return comp.Verify()
		}
		return comp.VerifyPackage(pkg)
	}

	return false
}

func printPlan(d diag.Sink, result *planResult, opts deployOptions) {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, opts)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPlanned changes:%v\n", colors.SpecUnimportant, colors.Reset))
	fmt.Printf(colors.Colorize(&prelude))

	// Print a nice message if the update is an empty one.
	if empty := checkEmpty(d, result.Plan); !empty {
		var summary bytes.Buffer
		step := result.Plan.Steps()
		counts := make(map[resource.StepOp]int)
		for step != nil {
			op := step.Op()
			// Print this step information (resource and all its properties).
			// TODO: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
			if opts.ShowReplaceSteps || (op != resource.OpReplaceCreate && op != resource.OpReplaceDelete) {
				printStep(&summary, step, opts.Summary, "")
			}
			counts[step.Op()]++
			step = step.Next()
		}

		// Print a summary of operation counts.
		printSummary(&summary, counts, opts.ShowReplaceSteps, true)
		fmt.Printf(colors.Colorize(&summary))
	}
}

func printPrelude(b *bytes.Buffer, result *planResult, opts deployOptions) {
	// If there are configuration variables, show them.
	if opts.ShowConfig {
		printConfig(b, result.compileResult)
	}

	// If show-sames was requested, walk the sames and print them.
	if opts.ShowUnchanged {
		printUnchanged(b, result.Plan, opts.Summary)
	}
}

func printConfig(b *bytes.Buffer, result *compileResult) {
	b.WriteString(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset))
	if result != nil && result.ConfigVars != nil {
		var toks []string
		for tok := range result.ConfigVars {
			toks = append(toks, string(tok))
		}
		sort.Strings(toks)
		for _, tok := range toks {
			b.WriteString(fmt.Sprintf("%v%v: %v\n", detailsIndent, tok, result.ConfigVars[tokens.Token(tok)]))
		}
	}
}

func printSummary(b *bytes.Buffer, counts map[resource.StepOp]int, showReplaceSteps bool, plan bool) {
	total := 0
	for op, c := range counts {
		if !showReplaceSteps && (op == resource.OpReplaceCreate || op == resource.OpReplaceDelete) {
			continue // skip counting replacement steps unless explicitly requested.
		}
		total += c
	}

	var planned string
	if plan {
		planned = "planned "
	}
	var colon string
	if total != 0 {
		colon = ":"
	}
	b.WriteString(fmt.Sprintf("%v total %v%v%v\n", total, planned, plural("change", total), colon))

	var planTo string
	var pastTense string
	if plan {
		planTo = "to "
	} else {
		pastTense = "d"
	}

	for _, op := range resource.StepOps() {
		if !showReplaceSteps && (op == resource.OpReplaceCreate || op == resource.OpReplaceDelete) {
			// Unless the user requested it, don't show the fine-grained replacement steps; just the logical ones.
			continue
		}
		if c := counts[op]; c > 0 {
			b.WriteString(fmt.Sprintf("    %v%v %v %v%v%v%v\n",
				op.Prefix(), c, plural("resource", c), planTo, op, pastTense, colors.Reset))
		}
	}
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}

const detailsIndent = "      " // 4 spaces, plus 2 for "+ ", "- ", and " " leaders

func printUnchanged(b *bytes.Buffer, plan resource.Plan, summary bool) {
	b.WriteString(fmt.Sprintf("%vUnchanged resources:%v\n", colors.SpecUnimportant, colors.Reset))
	for _, res := range plan.Unchanged() {
		b.WriteString("  ") // simulate the 2 spaces for +, -, etc.
		printResourceHeader(b, res, nil, "")
		printResourceProperties(b, res, nil, nil, nil, summary, "")
	}
}

func printStep(b *bytes.Buffer, step resource.Step, summary bool, indent string) {
	// First print out the operation's prefix.
	b.WriteString(step.Op().Prefix())

	// Next print the resource URN, properties, etc.
	printResourceHeader(b, step.Old(), step.New(), indent)
	b.WriteString(step.Op().Suffix())

	var replaces []resource.PropertyKey
	if step.Old() != nil {
		m := step.Old().URN()
		replaceMap := step.Plan().Replaces()
		replaces = replaceMap[m]
	}
	printResourceProperties(b, step.Old(), step.New(), step.NewProps(), replaces, summary, indent)

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printResourceHeader(b *bytes.Buffer, old resource.Resource, new resource.Resource, indent string) {
	var t tokens.Type
	if old == nil {
		t = new.Type()
	} else {
		t = old.Type()
	}

	// The primary header is the resource type (since it is easy on the eyes).
	b.WriteString(fmt.Sprintf("%s:\n", string(t)))
}

func printResourceProperties(b *bytes.Buffer, old resource.Resource, new resource.Resource,
	computed resource.PropertyMap, replaces []resource.PropertyKey, summary bool, indent string) {
	indent += detailsIndent

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	var URN resource.URN
	if old == nil {
		id = new.ID()
		URN = new.URN()
	} else {
		id = old.ID()
		URN = old.URN()
	}
	if id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}
	b.WriteString(fmt.Sprintf("%s[urn=%s]\n", indent, URN.Name()))

	if !summary {
		// Print all of the properties associated with this resource.
		if old == nil && new != nil {
			printObject(b, new.Properties(), indent)
		} else if new == nil && old != nil {
			printObject(b, old.Properties(), indent)
		} else {
			contract.Assert(computed != nil) // use computed properties for diffs.
			printOldNewDiffs(b, old.Properties(), computed, replaces, indent)
		}
	}
}

func maxKey(keys []resource.PropertyKey) int {
	maxkey := 0
	for _, k := range keys {
		if len(k) > maxkey {
			maxkey = len(k)
		}
	}
	return maxkey
}

func printObject(b *bytes.Buffer, props resource.PropertyMap, indent string) {
	// Compute the maximum with of property keys so we can justify everything.
	keys := resource.StablePropertyKeys(props)
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v, false) {
			printPropertyTitle(b, k, maxkey, indent)
			printPropertyValue(b, v, indent)
		}
	}
}

func printResourceOutputProperties(b *bytes.Buffer, step resource.Step, indent string) {
	indent += detailsIndent
	b.WriteString(step.Op().Color())
	b.WriteString(step.Op().Suffix())

	olds := step.Old().Properties()
	news := step.New().Properties()
	keys := resource.StablePropertyKeys(olds)
	maxkey := maxKey(keys)
	for _, k := range keys {
		v := news[k]
		if olds.NeedsValue(k) && shouldPrintPropertyValue(v, true) {
			printPropertyTitle(b, k, maxkey, indent)
			printPropertyValue(b, v, indent)
		}
	}

	b.WriteString(colors.Reset)
}

func shouldPrintPropertyValue(v resource.PropertyValue, outs bool) bool {
	if v.IsNull() {
		// by default, don't print nulls (they just clutter up the output)
		return false
	}
	if v.IsOutput() && !outs {
		// also don't show output properties until the outs parameter tells us to.
		return false
	}
	return true
}

func printPropertyTitle(b *bytes.Buffer, k resource.PropertyKey, align int, indent string) {
	b.WriteString(fmt.Sprintf("%s%-"+strconv.Itoa(align)+"s: ", indent, k))
}

func printPropertyValue(b *bytes.Buffer, v resource.PropertyValue, indent string) {
	if v.IsNull() {
		b.WriteString("<null>")
	} else if v.IsBool() {
		b.WriteString(fmt.Sprintf("%t", v.BoolValue()))
	} else if v.IsNumber() {
		b.WriteString(fmt.Sprintf("%v", v.NumberValue()))
	} else if v.IsString() {
		b.WriteString(fmt.Sprintf("%q", v.StringValue()))
	} else if v.IsResource() {
		b.WriteString(fmt.Sprintf("&%s", v.ResourceValue()))
	} else if v.IsArray() {
		b.WriteString(fmt.Sprintf("[\n"))
		for i, elem := range v.ArrayValue() {
			newIndent := printArrayElemHeader(b, i, indent)
			printPropertyValue(b, elem, newIndent)
		}
		b.WriteString(fmt.Sprintf("%s]", indent))
	} else if v.IsComputed() || v.IsOutput() {
		b.WriteString(v.TypeString())
	} else {
		contract.Assert(v.IsObject())
		b.WriteString("{\n")
		printObject(b, v.ObjectValue(), indent+"    ")
		b.WriteString(fmt.Sprintf("%s}", indent))
	}
	b.WriteString("\n")
}

func getArrayElemHeader(b *bytes.Buffer, i int, indent string) (string, string) {
	prefix := fmt.Sprintf("    %s[%d]: ", indent, i)
	return prefix, fmt.Sprintf("%-"+strconv.Itoa(len(prefix))+"s", "")
}

func printArrayElemHeader(b *bytes.Buffer, i int, indent string) string {
	prefix, newIndent := getArrayElemHeader(b, i, indent)
	b.WriteString(prefix)
	return newIndent
}

func printOldNewDiffs(b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap,
	replaces []resource.PropertyKey, indent string) {
	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, replaces, false, indent)
	} else {
		printObject(b, news, indent)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff,
	replaces []resource.PropertyKey, causedReplace bool, indent string) {
	contract.Assert(len(indent) > 2)

	// Compute the maximum with of property keys so we can justify everything.
	keys := diff.Keys()
	maxkey := maxKey(keys)

	// If a list of what causes a resource to get replaced exist, create a handy map.
	var replaceMap map[resource.PropertyKey]bool
	if len(replaces) > 0 {
		replaceMap = make(map[resource.PropertyKey]bool)
		for _, k := range replaces {
			replaceMap[k] = true
		}
	}

	// To print an object diff, enumerate the keys in stable order, and print each property independently.
	for _, k := range keys {
		title := func(id string) { printPropertyTitle(b, k, maxkey, id) }
		if add, isadd := diff.Adds[k]; isadd {
			if shouldPrintPropertyValue(add, false) {
				b.WriteString(colors.SpecAdded)
				title(addIndent(indent))
				printPropertyValue(b, add, addIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, false) {
				b.WriteString(colors.SpecDeleted)
				title(deleteIndent(indent))
				printPropertyValue(b, delete, deleteIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}
			printPropertyValueDiff(b, title, update, causedReplace, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same, false) {
			title(indent)
			printPropertyValue(b, diff.Sames[k], indent)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, title func(string), diff resource.ValueDiff,
	causedReplace bool, indent string) {
	contract.Assert(len(indent) > 2)

	if diff.Array != nil {
		title(indent)
		b.WriteString("[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			_, newIndent := getArrayElemHeader(b, i, indent)
			title := func(id string) { printArrayElemHeader(b, i, id) }
			if add, isadd := a.Adds[i]; isadd {
				b.WriteString(resource.OpCreate.Color())
				title(addIndent(indent))
				printPropertyValue(b, add, addIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				b.WriteString(resource.OpDelete.Color())
				title(deleteIndent(indent))
				printPropertyValue(b, delete, deleteIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if update, isupdate := a.Updates[i]; isupdate {
				title(indent)
				printPropertyValueDiff(b, func(string) {}, update, causedReplace, newIndent)
			} else {
				title(indent)
				printPropertyValue(b, a.Sames[i], newIndent)
			}
		}
		b.WriteString(fmt.Sprintf("%s]\n", indent))
	} else if diff.Object != nil {
		title(indent)
		b.WriteString("{\n")
		printObjectDiff(b, *diff.Object, nil, causedReplace, indent+"    ")
		b.WriteString(fmt.Sprintf("%s}\n", indent))
	} else if diff.Old.IsResource() && diff.New.IsResource() && diff.New.ResourceValue().Replacement() {
		// If the old and new are both resources, and the new is a replacement, show this in a special way (+-).
		b.WriteString(resource.OpReplace.Color())
		title(updateIndent(indent))
		printPropertyValue(b, diff.Old, updateIndent(indent))
		b.WriteString(colors.Reset)
	} else {
		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintPropertyValue(diff.Old, false) {
			var color string
			if causedReplace {
				color = resource.OpDelete.Color() // this property triggered replacement; color as a delete
			} else {
				color = resource.OpUpdate.Color()
			}
			b.WriteString(color)
			title(deleteIndent(indent))
			printPropertyValue(b, diff.Old, deleteIndent(indent))
			b.WriteString(colors.Reset)
		}
		if shouldPrintPropertyValue(diff.New, false) {
			var color string
			if causedReplace {
				color = resource.OpCreate.Color() // this property triggered replacement; color as a create
			} else {
				color = resource.OpUpdate.Color()
			}
			b.WriteString(color)
			title(addIndent(indent))
			printPropertyValue(b, diff.New, addIndent(indent))
			b.WriteString(colors.Reset)
		}
	}
}

func addIndent(indent string) string    { return indent[:len(indent)-2] + "+ " }
func deleteIndent(indent string) string { return indent[:len(indent)-2] + "- " }
func updateIndent(indent string) string { return indent[:len(indent)-2] + "+-" }

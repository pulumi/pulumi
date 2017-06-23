// Copyright 2016-2017, Pulumi Corporation
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
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/diag/colors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/deploy"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newPlanCmd() *cobra.Command {
	var analyzers []string
	var dotOutput bool
	var env string
	var showConfig bool
	var showReads bool
	var showReplaceDeletes bool
	var showSames bool
	var summary bool
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
			contract.Assertf(!dotOutput, "TODO[pulumi/lumi#235]: DOT files not yet supported")
			opts := deployOptions{
				Destroy:            false,
				DryRun:             true,
				Analyzers:          analyzers,
				ShowConfig:         showConfig,
				ShowReads:          showReads,
				ShowReplaceDeletes: showReplaceDeletes,
				ShowSames:          showSames,
				Summary:            summary,
				DOT:                dotOutput,
			}
			result, err := plan(cmd, info, opts)
			if err != nil {
				return err
			}
			if result != nil {
				defer contract.IgnoreClose(result)
				if err := printPlan(result, opts); err != nil {
					return err
				}
			}
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
		&showReads, "show-reads", false,
		"Show resources that will be read, in addition to those that will be modified")
	cmd.PersistentFlags().BoolVar(
		&showReplaceDeletes, "show-replace-deletes", false,
		"Show detailed resource replacement creates and deletes; normally shows as a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")

	return cmd
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, info *envCmdInfo, opts deployOptions) (*planResult, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Target != nil)

	// Create a context for plugins.
	ctx, err := plugin.NewContext(cmdutil.Diag(), nil)
	if err != nil {
		return nil, err
	}

	// First, compile the package, in preparatin for interpreting it and creating resources.
	result := compile(cmd, info.Args)
	if result == nil {
		return nil, nil
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/lumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	source := deploy.NewEvalSource(ctx, result.B.Ctx(), result.Pkg, nil, info.Target.Config, opts.Destroy)

	// If there are any analyzers in the project file, add them.
	var analyzers []tokens.QName
	if as := result.Pkg.Node.Analyzers; as != nil {
		for _, a := range *as {
			analyzers = append(analyzers, a)
		}
	}

	// Append any analyzers from the command line.
	for _, a := range opts.Analyzers {
		analyzers = append(analyzers, tokens.QName(a))
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan := deploy.NewPlan(ctx, info.Target, info.Snapshot, source, analyzers)
	return &planResult{
		Ctx:  ctx,
		Info: info,
		Plan: plan,
	}, nil
}

type planResult struct {
	Ctx  *plugin.Context // the context containing plugins and their state.
	Info *envCmdInfo     // plan command information.
	Plan *deploy.Plan    // the plan created by this command.
}

func (res *planResult) Close() error {
	return res.Ctx.Close()
}

func printPlan(result *planResult, opts deployOptions) error {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, opts, true)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPlanned changes:%v\n", colors.SpecUnimportant, colors.Reset))
	fmt.Print(colors.Colorize(&prelude))

	iter, err := result.Plan.Iterate()
	if err != nil {
		return errors.Errorf("An error occurred while preparing the plan: %v", err)
	}
	defer contract.IgnoreClose(iter)

	step, err := iter.Next()
	if err != nil {
		return errors.Errorf("An error occurred while enumerating the plan: %v", err)
	}

	var summary bytes.Buffer
	empty := true
	counts := make(map[deploy.StepOp]int)
	for step != nil {
		var err error

		// Perform the pre-step.
		if err = step.Pre(); err != nil {
			return errors.Errorf("An error occurred preparing the plan: %v", err)
		}

		// Print this step information (resource and all its properties).
		// IDEA: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
		track := shouldTrack(step, opts)
		if track {
			printStep(&summary, step, opts.Summary, true, "")
			empty = false
		}

		// Be sure to skip the step so that in-memory state updates are performed.
		if err = step.Skip(); err != nil {
			return errors.Errorf("An error occurred while advancing the plan: %v", err)
		}

		if track {
			counts[step.Op()]++
		}

		if step, err = iter.Next(); err != nil {
			return errors.Errorf("An error occurred while viewing the plan: %v", err)
		}
	}

	// If we are doing an empty update, say so.
	if empty {
		cmdutil.Diag().Infof(diag.Message("no resources need to be updated"))
	} else {
		// Print a summary of operation counts.
		printSummary(&summary, counts, true)
		fmt.Print(colors.Colorize(&summary))
	}
	return nil
}

// shouldTrack returns true if the step should be "tracked"; this affects two things: 1) whether the resource is shown
// in the planning phase and 2) whether the resource operation is tallied up and displayed in the final summary.
func shouldTrack(step deploy.Step, opts deployOptions) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if _, isrd := step.(deploy.ReadStep); isrd {
		return opts.ShowReads
	} else if step.Op() == deploy.OpSame {
		return opts.ShowSames
	} else if step.Op() == deploy.OpDelete && step.(*deploy.DeleteStep).Replaced() {
		return opts.ShowReplaceDeletes
	}
	// By default, however, steps are tracked.
	return true
}

func printPrelude(b *bytes.Buffer, result *planResult, opts deployOptions, planning bool) {
	// If there are configuration variables, show them.
	if opts.ShowConfig {
		printConfig(b, result.Info.Target.Config)
	}
}

func printConfig(b *bytes.Buffer, config resource.ConfigMap) {
	b.WriteString(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset))
	if config != nil {
		var toks []string
		for tok := range config {
			toks = append(toks, string(tok))
		}
		sort.Strings(toks)
		for _, tok := range toks {
			b.WriteString(fmt.Sprintf("%v%v: %v\n", detailsIndent, tok, config[tokens.Token(tok)]))
		}
	}
}

func printSummary(b *bytes.Buffer, counts map[deploy.StepOp]int, plan bool) {
	total := 0
	for _, c := range counts {
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

	for _, op := range deploy.StepOps {
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

func printStep(b *bytes.Buffer, step deploy.Step, summary bool, planning bool, indent string) {
	// First print out the operation's prefix.
	b.WriteString(step.Op().Prefix())

	// Next, print the resource type (since it is easy on the eyes and can be quickly identified).
	printStepHeader(b, step)
	b.WriteString(step.Op().Suffix())

	// Next print the resource URN, properties, etc.
	if mut, ismut := step.(deploy.MutatingStep); ismut {
		var replaces []resource.PropertyKey
		if step.Op() == deploy.OpReplace {
			replaces = step.(*deploy.ReplaceStep).Reasons()
		}
		printResourceProperties(b,
			mut.URN(), mut.Old(), mut.New(), mut.Inputs(), replaces, summary, planning, indent)
	} else if rd, isrd := step.(deploy.ReadStep); isrd {
		for _, res := range rd.Resources() {
			printResourceProperties(b,
				"", nil, res, res.CopyProperties(), nil, summary, planning, indent)
		}
	} else {
		contract.Failf("Expected each step to either be mutating or read-only")
	}

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printStepHeader(b *bytes.Buffer, step deploy.Step) {
	b.WriteString(fmt.Sprintf("%s:\n", string(step.Type())))
}

func printResourceProperties(b *bytes.Buffer, urn resource.URN, old *resource.State, new *resource.Object,
	props resource.PropertyMap, replaces []resource.PropertyKey, summary bool, planning bool, indent string) {
	indent += detailsIndent

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	if old != nil {
		id = old.ID()
	}
	if id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}
	if urn != "" {
		b.WriteString(fmt.Sprintf("%s[urn=%s]\n", indent, urn))
	}

	if !summary {
		// Print all of the properties associated with this resource.
		if old == nil && new != nil {
			printObject(b, props, planning, indent)
		} else if new == nil && old != nil {
			printObject(b, old.Inputs(), planning, indent)
		} else {
			contract.Assert(props != nil) // use computed properties for diffs.
			printOldNewDiffs(b, old.Inputs(), props, replaces, planning, indent)
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

func printObject(b *bytes.Buffer, props resource.PropertyMap, planning bool, indent string) {
	// Compute the maximum with of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v, planning) {
			printPropertyTitle(b, k, maxkey, indent)
			printPropertyValue(b, v, planning, indent)
		}
	}
}

// printResourceOutputProperties prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func printResourceOutputProperties(b *bytes.Buffer, step deploy.Step, indent string) {
	mut, ismut := step.(deploy.MutatingStep)
	if !ismut {
		// Only mutating steps have output properties associated with them.
		return
	}

	indent += detailsIndent
	b.WriteString(step.Op().Color())
	b.WriteString(step.Op().Suffix())

	// First fetch all the relevant property maps that we may consult.
	newins := mut.Inputs()
	newouts := mut.Outputs()
	var oldouts resource.PropertyMap
	if old := mut.Old(); old != nil {
		oldouts = old.Outputs()
	}

	// Now sort the keys and enumerate each output property in a deterministic order.
	keys := newouts.StableKeys()
	maxkey := maxKey(keys)
	for _, k := range keys {
		newout := newouts[k]
		// Print this property if it is printable, and one of these cases
		//     1) new ins has it and it's different;
		//     2) new ins doesn't have it, but old outs does, and it's different;
		//     3) neither old outs nor new ins contain it;
		if shouldPrintPropertyValue(newout, true) {
			var print bool
			if newin, has := newins[k]; has {
				print = (newout.Diff(newin) != nil) // case 1
			} else if oldouts != nil {
				if oldout, has := oldouts[k]; has {
					print = (newout.Diff(oldout) != nil) // case 2
				} else {
					print = true // case 3
				}
			} else {
				print = true // also case 3
			}

			if print {
				printPropertyTitle(b, k, maxkey, indent)
				printPropertyValue(b, newout, false, indent)
			}
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

func printPropertyValue(b *bytes.Buffer, v resource.PropertyValue, planning bool, indent string) {
	if v.IsNull() {
		b.WriteString("<null>")
	} else if v.IsBool() {
		b.WriteString(fmt.Sprintf("%t", v.BoolValue()))
	} else if v.IsNumber() {
		b.WriteString(fmt.Sprintf("%v", v.NumberValue()))
	} else if v.IsString() {
		b.WriteString(fmt.Sprintf("%q", v.StringValue()))
	} else if v.IsArray() {
		b.WriteString(fmt.Sprintf("[\n"))
		for i, elem := range v.ArrayValue() {
			newIndent := printArrayElemHeader(b, i, indent)
			printPropertyValue(b, elem, planning, newIndent)
		}
		b.WriteString(fmt.Sprintf("%s]", indent))
	} else if v.IsComputed() || v.IsOutput() {
		b.WriteString(v.TypeString())
	} else {
		contract.Assert(v.IsObject())
		b.WriteString("{\n")
		printObject(b, v.ObjectValue(), planning, indent+"    ")
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
	replaces []resource.PropertyKey, planning bool, indent string) {
	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, replaces, false, planning, indent)
	} else {
		printObject(b, news, planning, indent)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff,
	replaces []resource.PropertyKey, causedReplace bool, planning bool, indent string) {
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
			if shouldPrintPropertyValue(add, planning) {
				b.WriteString(colors.SpecAdded)
				title(addIndent(indent))
				printPropertyValue(b, add, planning, addIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				b.WriteString(colors.SpecDeleted)
				title(deleteIndent(indent))
				printPropertyValue(b, delete, planning, deleteIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}
			printPropertyValueDiff(b, title, update, causedReplace, planning, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same, planning) {
			title(indent)
			printPropertyValue(b, diff.Sames[k], planning, indent)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, title func(string), diff resource.ValueDiff,
	causedReplace bool, planning bool, indent string) {
	contract.Assert(len(indent) > 2)

	if diff.Array != nil {
		title(indent)
		b.WriteString("[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			_, newIndent := getArrayElemHeader(b, i, indent)
			titleFunc := func(id string) { printArrayElemHeader(b, i, id) }
			if add, isadd := a.Adds[i]; isadd {
				b.WriteString(deploy.OpCreate.Color())
				titleFunc(addIndent(indent))
				printPropertyValue(b, add, planning, addIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				b.WriteString(deploy.OpDelete.Color())
				titleFunc(deleteIndent(indent))
				printPropertyValue(b, delete, planning, deleteIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(b, title, update, causedReplace, planning, indent)
			} else {
				titleFunc(indent)
				printPropertyValue(b, a.Sames[i], planning, newIndent)
			}
		}
		b.WriteString(fmt.Sprintf("%s]\n", indent))
	} else if diff.Object != nil {
		title(indent)
		b.WriteString("{\n")
		printObjectDiff(b, *diff.Object, nil, causedReplace, planning, indent+"    ")
		b.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintPropertyValue(diff.Old, false) {
			var color string
			if causedReplace {
				color = deploy.OpDelete.Color() // this property triggered replacement; color as a delete
			} else {
				color = deploy.OpUpdate.Color()
			}
			b.WriteString(color)
			title(deleteIndent(indent))
			printPropertyValue(b, diff.Old, planning, deleteIndent(indent))
			b.WriteString(colors.Reset)
		}
		if shouldPrintPropertyValue(diff.New, false) {
			var color string
			if causedReplace {
				color = deploy.OpCreate.Color() // this property triggered replacement; color as a create
			} else {
				color = deploy.OpUpdate.Color()
			}
			b.WriteString(color)
			title(addIndent(indent))
			printPropertyValue(b, diff.New, planning, addIndent(indent))
			b.WriteString(colors.Reset)
		}
	}
}

func addIndent(indent string) string    { return indent[:len(indent)-2] + "+ " }
func deleteIndent(indent string) string { return indent[:len(indent)-2] + "- " }

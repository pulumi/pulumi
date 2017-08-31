// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/diag/colors"
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/resource/deploy"
	"github.com/pulumi/pulumi-fabric/pkg/resource/plugin"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type PlanOptions struct {
	Package              string   // the package to compute the plan for
	Environment          string   // the environment to use when planning
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	Debug                bool     // true to enable resource debugging output.
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
}

func (eng *Engine) Plan(opts PlanOptions) error {
	// Initialize the diagnostics logger with the right stuff.
	eng.InitDiag(diag.FormatOptions{
		Colors: true,
		Debug:  opts.Debug,
	})

	info, err := eng.initEnvCmdName(tokens.QName(opts.Environment), opts.Package)
	if err != nil {
		return err
	}
	deployOpts := deployOptions{
		Debug:                opts.Debug,
		Destroy:              false,
		DryRun:               true,
		Analyzers:            opts.Analyzers,
		ShowConfig:           opts.ShowConfig,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
	}
	result, err := eng.plan(info, deployOpts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)
		if err := eng.printPlan(result, deployOpts); err != nil {
			return err
		}
	}
	return nil
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func (eng *Engine) plan(info *envCmdInfo, opts deployOptions) (*planResult, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Target != nil)

	// Create a context for plugins.
	ctx, err := plugin.NewContext(eng.Diag(), nil)
	if err != nil {
		return nil, err
	}

	// First, load the package metadata, in preparation for executing it and creating resources.
	pkginfo, err := eng.readPackageFromArg(info.PackageArg)
	if err != nil {
		return nil, errors.Errorf("Error loading package: %v", err)
	}
	contract.Assert(pkginfo != nil)

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi-fabric#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	source := deploy.NewEvalSource(ctx, &deploy.EvalRunInfo{
		Pkg:    pkginfo.Pkg,
		Pwd:    pkginfo.Root,
		Config: info.Target.Config,
	}, opts.Destroy, opts.DryRun)

	// If there are any analyzers in the project file, add them.
	var analyzers []tokens.QName
	if as := pkginfo.Pkg.Analyzers; as != nil {
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

func (eng *Engine) printPlan(result *planResult, opts deployOptions) error {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, opts, true)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPlanning changes:%v\n", colors.SpecUnimportant, colors.Reset))
	fmt.Fprint(eng.Stdout, colors.Colorize(&prelude))

	iter, err := result.Plan.Start()
	if err != nil {
		return errors.Errorf("An error occurred while preparing the plan: %v", err)
	}
	defer contract.IgnoreClose(iter)

	step, err := iter.Next()
	if err != nil {
		return errors.Errorf("An error occurred while enumerating the plan: %v", err)
	}

	var summary bytes.Buffer
	counts := make(map[deploy.StepOp]int)
	for step != nil {
		var err error

		// Perform the pre-step.
		if err = step.Pre(); err != nil {
			return errors.Errorf("An error occurred preparing the plan: %v", err)
		}

		// Print this step information (resource and all its properties).
		// IDEA: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
		if shouldShow(step, opts) {
			printStep(&summary, step, opts.Summary, true, "")
		}

		// Be sure to skip the step so that in-memory state updates are performed.
		if err = step.Skip(); err != nil {
			return errors.Errorf("An error occurred while advancing the plan: %v", err)
		}

		// Track the operation if shown and/or if it is a logically meaningful operation.
		if step.Logical() {
			counts[step.Op()]++
		}

		if step, err = iter.Next(); err != nil {
			return errors.Errorf("An error occurred while viewing the plan: %v", err)
		}
	}

	// Print a summary of operation counts.
	printChangeSummary(&summary, counts, true)
	fmt.Fprint(eng.Stdout, colors.Colorize(&summary))
	return nil
}

// shouldShow returns true if a step should show in the output.
func shouldShow(step deploy.Step, opts deployOptions) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if step.Op() == deploy.OpSame {
		return opts.ShowSames
	} else if step.Op() == deploy.OpCreateReplacement || step.Op() == deploy.OpDeleteReplaced {
		return opts.ShowReplacementSteps
	} else if step.Op() == deploy.OpReplace {
		return !opts.ShowReplacementSteps
	}
	return true
}

func printPrelude(b *bytes.Buffer, result *planResult, opts deployOptions, planning bool) {
	// If there are configuration variables, show them.
	if opts.ShowConfig {
		printConfig(b, result.Info.Target.Config)
	}
}

func printConfig(b *bytes.Buffer, config map[tokens.ModuleMember]string) {
	b.WriteString(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset))
	if config != nil {
		var keys []string
		for key := range config {
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("%v%v: %v\n", detailsIndent, key, config[tokens.ModuleMember(key)]))
		}
	}
}

func printChangeSummary(b *bytes.Buffer, counts map[deploy.StepOp]int, plan bool) int {
	changes := 0
	for op, c := range counts {
		if op != deploy.OpSame {
			changes += c
		}
	}

	var kind string
	if plan {
		kind = "planned"
	} else {
		kind = "deployed"
	}

	var changesLabel string
	if changes == 0 {
		kind = "required"
		changesLabel = "no"
	} else {
		changesLabel = strconv.Itoa(changes)
	}

	b.WriteString(fmt.Sprintf("%vinfo%v: %v %v %v:\n",
		colors.SpecInfo, colors.Reset, changesLabel, plural("change", changes), kind))

	var planTo string
	var pastTense string
	if plan {
		planTo = "to "
	} else {
		pastTense = "d"
	}

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range deploy.StepOps {
		if op != deploy.OpSame {
			if c := counts[op]; c > 0 {
				b.WriteString(fmt.Sprintf("    %v%v %v %v%v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, op, pastTense, colors.Reset))
			}
		}
	}
	if c := counts[deploy.OpSame]; c > 0 {
		b.WriteString(fmt.Sprintf("      %v %v unchanged\n", c, plural("resource", c)))
	}

	return changes
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
		if step.Op() == deploy.OpCreateReplacement {
			replaces = step.(*deploy.CreateStep).Keys()
		} else if step.Op() == deploy.OpReplace {
			replaces = step.(*deploy.ReplaceStep).Keys()
		}
		printResourceProperties(b, mut.URN(), mut.Old(), mut.New(), replaces, summary, planning, indent)
	} else {
		contract.Failf("Expected each step to either be mutating or read-only")
	}

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printStepHeader(b *bytes.Buffer, step deploy.Step) {
	b.WriteString(fmt.Sprintf("%s: (%s)\n", string(step.Type()), step.Op()))
}

func printResourceProperties(b *bytes.Buffer, urn resource.URN, old *resource.State, new *resource.State,
	replaces []resource.PropertyKey, summary bool, planning bool, indent string) {
	indent += detailsIndent

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	if old != nil {
		id = old.ID
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
			printObject(b, new.AllInputs(), planning, indent)
		} else if new == nil && old != nil {
			printObject(b, old.AllInputs(), planning, indent)
		} else {
			printOldNewDiffs(b, old.AllInputs(), new.AllInputs(), replaces, planning, indent)
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
	// Only certain kinds of steps have output properties associated with them.
	mut := step.(deploy.MutatingStep)
	if mut == nil ||
		(step.Op() != deploy.OpCreate &&
			step.Op() != deploy.OpCreateReplacement &&
			step.Op() != deploy.OpUpdate) {
		return
	}

	indent += detailsIndent
	b.WriteString(step.Op().Color())
	b.WriteString(step.Op().Suffix())

	// First fetch all the relevant property maps that we may consult.
	newins := mut.New().Inputs
	newouts := mut.New().Outputs
	var oldouts resource.PropertyMap
	if old := mut.Old(); old != nil {
		oldouts = old.Outputs
	}

	// Now sort the keys and enumerate each output property in a deterministic order.
	firstout := true
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
				if firstout {
					b.WriteString(fmt.Sprintf("%v---outputs:---\n", indent))
					firstout = false
				}
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
		arr := v.ArrayValue()
		if len(arr) == 0 {
			b.WriteString("[]")
		} else {
			b.WriteString(fmt.Sprintf("[\n"))
			for i, elem := range arr {
				newIndent := printArrayElemHeader(b, i, indent)
				printPropertyValue(b, elem, planning, newIndent)
			}
			b.WriteString(fmt.Sprintf("%s]", indent))
		}
	} else if v.IsAsset() {
		a := v.AssetValue()
		if text, has := a.GetText(); has {
			b.WriteString("asset {\n")
			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				b.WriteString(fmt.Sprintf("%v    \"%v\"\n", indent, line))
			}
			b.WriteString(fmt.Sprintf("%v}", indent))
		} else if path, has := a.GetPath(); has {
			b.WriteString(fmt.Sprintf("asset { file://%v }", path))
		} else {
			contract.Assert(a.IsURI())
			b.WriteString(fmt.Sprintf("asset { %v }", a.URI))
		}
	} else if v.IsArchive() {
		a := v.ArchiveValue()
		if assets, has := a.GetAssets(); has {
			b.WriteString("archive {\n")
			var names []string
			for name := range assets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				b.WriteString(fmt.Sprintf("%v    \"%v\": ", indent, name))
				printPropertyValue(b, resource.NewAssetProperty(assets[name]), planning, indent+"    ")
			}
			b.WriteString(fmt.Sprintf("%v}", indent))
		} else if path, has := a.GetPath(); has {
			b.WriteString(fmt.Sprintf("archive { file://%v }", path))
		} else {
			contract.Assert(a.IsURI())
			b.WriteString(fmt.Sprintf("archive { %v }", a.URI))
		}
	} else if v.IsComputed() || v.IsOutput() {
		b.WriteString(v.TypeString())
	} else {
		contract.Assert(v.IsObject())
		obj := v.ObjectValue()
		if len(obj) == 0 {
			b.WriteString("{}")
		} else {
			b.WriteString("{\n")
			printObject(b, obj, planning, indent+"    ")
			b.WriteString(fmt.Sprintf("%s}", indent))
		}
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
				b.WriteString(colors.SpecCreate)
				title(addIndent(indent))
				printPropertyValue(b, add, planning, addIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				b.WriteString(colors.SpecDelete)
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

// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func (eng *Engine) plan(info *planContext, opts deployOptions) (*planResult, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Target != nil)

	// Create a context for plugins.
	ctx, err := plugin.NewContext(opts.Diag, nil, info.TracingSpan)
	if err != nil {
		return nil, err
	}

	// First, load the package metadata, in preparation for executing it and creating resources.
	pkginfo, err := ReadPackageFromArg(info.PackageArg)
	if err != nil {
		return nil, errors.Errorf("Error loading package: %v", err)
	}
	contract.Assert(pkginfo != nil)

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := pkginfo.GetPwdMain()
	if err != nil {
		return nil, err
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	source := deploy.NewEvalSource(ctx, &deploy.EvalRunInfo{
		Pkg:     pkginfo.Pkg,
		Pwd:     pwd,
		Program: main,
		Target:  info.Target,
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
		Ctx:     ctx,
		Info:    info,
		Plan:    plan,
		Options: opts,
	}, nil
}

type planResult struct {
	Ctx     *plugin.Context // the context containing plugins and their state.
	Info    *planContext    // plan command information.
	Plan    *deploy.Plan    // the plan created by this command.
	Options deployOptions   // the deployment options.
}

// StepActions is used to process a plan's steps.
type StepActions interface {
	// Run is invoked to perform whatever action the implementer uses to process the step.
	Run(step deploy.Step) (resource.Status, error)
}

// Walk enumerates all steps in the plan, calling out to the provided action at each step.  It returns four things: the
// resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step that
// failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (res *planResult) Walk(actions StepActions) (deploy.PlanSummary, deploy.Step, resource.Status, error) {
	opts := deploy.Options{
		Parallel: res.Options.Parallel,
	}

	// Fetch a plan iterator and keep walking it until we are done.
	iter, err := res.Plan.Start(opts)
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}

	step, err := iter.Next()
	if err != nil {
		_ = iter.Close() // ignore close errors; the Next error trumps
		return nil, nil, resource.StatusOK, err
	}

	for step != nil {
		// Perform any per-step actions.
		rst, err := actions.Run(step)

		// If an error occurred, exit early.
		if err != nil {
			_ = iter.Close() // ignore close errors; the action error trumps
			return iter, step, rst, err
		}
		contract.Assert(rst == resource.StatusOK)

		step, err = iter.Next()
		if err != nil {
			_ = iter.Close() // ignore close errors; the action error trumps
			return iter, step, resource.StatusOK, err
		}
	}

	// Finally, return a summary and the resulting plan information.
	return iter, nil, resource.StatusOK, iter.Close()
}

func (res *planResult) Close() error {
	return res.Ctx.Close()
}

type previewActions struct {
	Summary bytes.Buffer
	Ops     map[deploy.StepOp]int
	Opts    deployOptions
}

func (acts *previewActions) Run(step deploy.Step) (resource.Status, error) {
	// Print this step information (resource and all its properties).
	if shouldShow(step, acts.Opts) {
		printStep(&acts.Summary, step, acts.Opts.Summary, true, "")
	}

	// Be sure to skip the step so that in-memory state updates are performed.
	err := step.Skip()

	// We let `printPlan` handle error reporting for now.
	if err == nil {
		// Track the operation if shown and/or if it is a logically meaningful operation.
		if step.Logical() {
			acts.Ops[step.Op()]++
		}
	}

	return resource.StatusOK, err
}

func (eng *Engine) printPlan(result *planResult) error {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, true)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPreviewing changes:%v\n", colors.SpecUnimportant, colors.Reset))
	result.Options.Events <- stdOutEventWithColor(&prelude)

	actions := &previewActions{
		Ops:  make(map[deploy.StepOp]int),
		Opts: result.Options,
	}
	_, _, _, err := result.Walk(actions)
	if err != nil {
		return errors.Errorf("An error occurred while advancing the preview: %v", err)
	}

	if !result.Options.Diag.Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return errors.New("One or more errors occurred during this preview")
	}

	// Print a summary of operation counts.
	printChangeSummary(&actions.Summary, actions.Ops, true)
	result.Options.Events <- stdOutEventWithColor(&actions.Summary)
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

func printPrelude(b *bytes.Buffer, result *planResult, planning bool) {
	// If there are configuration variables, show them.
	if result.Options.ShowConfig {
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

func printChangeSummary(b *bytes.Buffer, counts map[deploy.StepOp]int, preview bool) int {
	changes := 0
	for op, c := range counts {
		if op != deploy.OpSame {
			changes += c
		}
	}

	var kind string
	if preview {
		kind = "previewed"
	} else {
		kind = "performed"
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
	if preview {
		planTo = "to "
	}

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range deploy.StepOps {
		if op != deploy.OpSame {
			if c := counts[op]; c > 0 {
				opDescription := string(op)
				if !preview {
					opDescription = op.PastTense()
				}
				b.WriteString(fmt.Sprintf("    %v%v %v %v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, opDescription, colors.Reset))
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

	// If this resource has children, also print a summary of those out too.
	var children []resource.URN
	if new != nil {
		children = new.Children
	} else {
		children = old.Children
	}
	for _, child := range children {
		b.WriteString(fmt.Sprintf("%s=> %s\n", indent, child))
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
			printPropertyTitle(b, string(k), maxkey, indent)
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
				printPropertyTitle(b, string(k), maxkey, indent)
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

func printPropertyTitle(b *bytes.Buffer, name string, align int, indent string) {
	b.WriteString(fmt.Sprintf("%s%-"+strconv.Itoa(align)+"s: ", indent, name))
}

func printPropertyValue(
	b *bytes.Buffer, v resource.PropertyValue, planning bool,
	indent string) {

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
			b.WriteString(fmt.Sprintf("asset(text:%s) {\n", shortHash(a.Hash)))

			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				b.WriteString(fmt.Sprintf("%s    \"%s\"\n", indent, line))
			}
			b.WriteString(fmt.Sprintf("%v}", indent))
		} else if path, has := a.GetPath(); has {
			b.WriteString(fmt.Sprintf("asset(file:%s) { %s }", shortHash(a.Hash), path))
		} else {
			contract.Assert(a.IsURI())
			b.WriteString(fmt.Sprintf("asset(uri:%s) { %s }", shortHash(a.Hash), a.URI))
		}
	} else if v.IsArchive() {
		a := v.ArchiveValue()
		if assets, has := a.GetAssets(); has {
			b.WriteString(fmt.Sprintf("archive(assets:%s) {\n", shortHash(a.Hash)))
			var names []string
			for name := range assets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				printAssetOrArchive(b, assets[name], name, indent, planning)
			}
			b.WriteString(fmt.Sprintf("%v}", indent))
		} else if path, has := a.GetPath(); has {
			b.WriteString(fmt.Sprintf("archive(file:%s) { %s }", shortHash(a.Hash), path))
		} else {
			contract.Assert(a.IsURI())
			b.WriteString(fmt.Sprintf("archive(uri:%s) { %v }", shortHash(a.Hash), a.URI))
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

func printAssetOrArchive(b *bytes.Buffer, v interface{}, name string, indent string, planning bool) {
	b.WriteString(fmt.Sprintf("%v    \"%v\": ", indent, name))
	printPropertyValue(b, assetOrArchiveToPropertyValue(v), planning, indent+"    ")
}

func assetOrArchiveToPropertyValue(v interface{}) resource.PropertyValue {
	switch t := v.(type) {
	case *resource.Asset:
		return resource.NewAssetProperty(t)
	case *resource.Archive:
		return resource.NewArchiveProperty(t)
	default:
		contract.Failf("Unexpected archive element '%v'", reflect.TypeOf(t))
		return resource.PropertyValue{V: nil}
	}
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
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

func printOldNewDiffs(
	b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap,
	replaces []resource.PropertyKey, planning bool, indent string) {

	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, replaces, false, planning, indent)
	} else {
		printObject(b, news, planning, indent)
	}
}

func printObjectDiff(
	b *bytes.Buffer, diff resource.ObjectDiff,
	replaces []resource.PropertyKey, causedReplace bool, planning bool,
	indent string) {

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
		title := func(id string) { printPropertyTitle(b, string(k), maxkey, id) }
		if add, isadd := diff.Adds[k]; isadd {
			if shouldPrintPropertyValue(add, planning) {
				printAdd(b, add, title, true, planning, indent, indent)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				printDelete(b, delete, title, true, planning, indent, indent)
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

func printPropertyValueDiff(
	b *bytes.Buffer, title func(string), diff resource.ValueDiff,
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
				printAdd(b, add, titleFunc, true, planning, indent, newIndent)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				printDelete(b, delete, titleFunc, true, planning, indent, newIndent)
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
		shouldPrintOld := shouldPrintPropertyValue(diff.Old, false)
		shouldPrintNew := shouldPrintPropertyValue(diff.New, false)

		if diff.Old.IsArchive() &&
			diff.New.IsArchive() &&
			!causedReplace &&
			shouldPrintOld &&
			shouldPrintNew {
			printArchiveDiff(b, title, diff.Old.ArchiveValue(), diff.New.ArchiveValue(), planning, indent)
			return
		}

		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintOld {
			printDelete(b, diff.Old, title, causedReplace, planning, indent, indent)
		}
		if shouldPrintNew {
			printAdd(b, diff.New, title, causedReplace, planning, indent, indent)
		}
	}
}

func printDelete(
	b *bytes.Buffer, v resource.PropertyValue, title func(string), causedReplace bool,
	planning bool, indent string, newIndent string) {

	var color string
	if causedReplace {
		color = colors.SpecDelete
	} else {
		color = colors.SpecUpdate
	}
	b.WriteString(color)
	title(deletedIndentString(indent))
	printPropertyValue(b, v, planning, deletedIndentString(newIndent))
	b.WriteString(colors.Reset)
}

func printAdd(
	b *bytes.Buffer, v resource.PropertyValue, title func(string), causedReplace bool,
	planning bool, indent string, newIndent string) {

	var color string
	if causedReplace {
		color = colors.SpecCreate
	} else {
		color = colors.SpecUpdate
	}

	b.WriteString(color)
	title(addedIndentString(indent))
	printPropertyValue(b, v, planning, addedIndentString(newIndent))
	b.WriteString(colors.Reset)
}

func printArchiveDiff(
	b *bytes.Buffer, title func(string),
	oldArchive *resource.Archive, newArchive *resource.Archive,
	planning bool, indent string) {

	// TODO: this could be called recursively from itself.  In the recursive case, we might have an
	// archive that actually hasn't changed.  Check for that, and terminate the diff printing.

	color := deploy.OpUpdate.Color()
	b.WriteString(color)
	title(changedIndentString(indent))

	hashChange := getTextChangeString(shortHash(oldArchive.Hash), shortHash(newArchive.Hash))
	if oldPath, has := oldArchive.GetPath(); has {
		newPath, has := newArchive.GetPath()
		contract.Assert(has)

		b.WriteString(fmt.Sprintf("archive(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath)))
	} else if oldURI, has := oldArchive.GetURI(); has {
		newURI, has := newArchive.GetURI()
		contract.Assert(has)

		b.WriteString(fmt.Sprintf("archive(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI)))
	} else {
		contract.Assert(oldArchive.IsAssets() && newArchive.IsAssets())
		b.WriteString(fmt.Sprintf("archive(assets:%s) {\n", hashChange))

		oldAssets, _ := oldArchive.GetAssets()
		newAssets, _ := newArchive.GetAssets()

		printAssetsDiff(b, oldAssets, newAssets, planning, indent+"    ")

		b.WriteString(fmt.Sprintf("%v}\n", indent))
	}

	b.WriteString(colors.Reset)
}

func printAssetsDiff(
	b *bytes.Buffer,
	oldAssets map[string]interface{}, newAssets map[string]interface{},
	planning bool, indent string) {

	var oldNames []string
	var newNames []string

	for name := range oldAssets {
		oldNames = append(oldNames, name)
	}

	for name := range newAssets {
		newNames = append(newNames, name)
	}

	sort.Strings(oldNames)
	sort.Strings(newNames)

	i := 0
	j := 0

	var keys []resource.PropertyKey
	for _, name := range oldNames {
		keys = append(keys, "\""+resource.PropertyKey(name)+"\"")
	}
	for _, name := range newNames {
		keys = append(keys, "\""+resource.PropertyKey(name)+"\"")
	}

	maxkey := maxKey(keys)

	for i < len(oldNames) || j < len(newNames) {
		color := deploy.OpUpdate.Color()
		b.WriteString(color)

		deleteOld := false
		addNew := false
		if i < len(oldNames) && j < len(newNames) {
			oldName := oldNames[i]
			newName := newNames[j]

			if oldName == newName {
				title := func(id string) { printPropertyTitle(b, "\""+oldName+"\"", maxkey, id) }

				oldAsset := oldAssets[oldName]
				newAsset := newAssets[newName]

				// b.WriteString(fmt.Sprintf("%v\"%v\": ", indent, oldName))

				switch t := oldAsset.(type) {
				case *resource.Archive:
					printArchiveDiff(b, title, t, newAsset.(*resource.Archive), planning, indent+"    ")
					break
				case *resource.Asset:
					printAssetDiff(b, title, t, newAsset.(*resource.Asset), planning, indent+"    ")
					break
				}
				i++
				j++
				continue
			}

			if oldName < newName {
				deleteOld = true
			} else {
				addNew = true
			}
		} else if i < len(oldNames) {
			deleteOld = true
		} else {
			addNew = true
		}

		newIndent := indent + "    "
		if deleteOld {
			oldName := oldNames[i]
			title := func(id string) { printPropertyTitle(b, "\""+oldName+"\"", maxkey, id) }
			printDelete(b, assetOrArchiveToPropertyValue(oldAssets[oldName]), title, false, planning, newIndent, newIndent)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			title := func(id string) { printPropertyTitle(b, "\""+newName+"\"", maxkey, id) }
			printAdd(b, assetOrArchiveToPropertyValue(newAssets[newName]), title, false, planning, newIndent, newIndent)
			j++
		}

		b.WriteString(colors.Reset)
	}
}

func printAssetDiff(
	b *bytes.Buffer, title func(string),
	oldAsset *resource.Asset, newAsset *resource.Asset,
	planning bool, indent string) {

	// If the assets aren't changed, just print out: = assetName: type(hash)
	if oldAsset.Hash == newAsset.Hash {
		b.WriteString(colors.Reset)
		title(unchangedIndentString(indent))

		hash := shortHash(oldAsset.Hash)
		if path, has := oldAsset.GetPath(); has {
			b.WriteString(fmt.Sprintf("asset(file:%s) { %s }\n", hash, path))
		} else if uri, has := oldAsset.GetURI(); has {
			b.WriteString(fmt.Sprintf("asset(uri:%s) { %s }\n", hash, uri))
		} else {
			b.WriteString(fmt.Sprintf("asset(text:%s)\n", hash))
		}

		b.WriteString(deploy.OpUpdate.Color())
		return
	}

	// if the asset changed, print out: ~ assetName: type(hash->hash) details...
	title(changedIndentString(indent))

	hashChange := getTextChangeString(shortHash(oldAsset.Hash), shortHash(newAsset.Hash))

	if oldText, has := oldAsset.GetText(); has {
		newText, has := newAsset.GetText()
		contract.Assert(has)

		b.WriteString(fmt.Sprintf("asset(text:%s) {\n\n", hashChange))

		massagedOldText := massageText(oldText)
		massagedNewText := massageText(newText)

		differ := diffmatchpatch.New()
		differ.DiffTimeout = 0

		hashed1, hashed2, lineArray := differ.DiffLinesToChars(massagedOldText, massagedNewText)
		diffs1 := differ.DiffMain(hashed1, hashed2, false)
		diffs2 := differ.DiffCharsToLines(diffs1, lineArray)

		b.WriteString(diffToPrettyString(diffs2))
		b.WriteString(fmt.Sprintf("\n%v}\n", indent))
	} else if oldPath, has := oldAsset.GetPath(); has {
		newPath, has := newAsset.GetPath()
		contract.Assert(has)

		b.WriteString(fmt.Sprintf("asset(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath)))
	} else {
		contract.Assert(oldAsset.IsURI())

		oldURI, has := oldAsset.GetURI()
		contract.Assert(has)
		newURI, has := newAsset.GetURI()
		contract.Assert(has)

		b.WriteString(fmt.Sprintf("asset(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI)))
	}
}

func getTextChangeString(old string, new string) string {
	if old == new {
		return old
	}

	return fmt.Sprintf("%s->%s", old, new)
}

// massageText takes the text for a function and cleans it up a bit to make the user visible diffs
// less noisy.  Specifically:
//   1. it tries to condense things by changling multiple blank lines into a single blank line.
//   2. it normalizs the sha hashes we emit so that changes to them don't appear in the diff.
//   3. it elides the with-capture headers, as changes there are not generally meaningful.
func massageText(text string) string {
	for true {
		newText := strings.Replace(text, "\n\n\n", "\n\n", -1)
		if len(newText) == len(text) {
			break
		}

		text = newText
	}

	shaRegexp, _ := regexp.Compile("__[a-zA-Z0-9]{40}")
	closureRegexp, _ := regexp.Compile("    with\\(\\{ .* \\}\\) \\{")

	text = shaRegexp.ReplaceAllString(text, "__shaHash")
	text = closureRegexp.ReplaceAllString(text, "    with (__closure) {")

	return text
}

// diffToPrettyString takes the full diff produed by diffmatchpatch and condenses it into something
// useful we can print to the console.  Specifically, while it includes any adds/removes in
// green/red, it will also show portions of the unchanged text to help give surrounding context to
// those add/removes. Because the unchanged portions may be very large, it only included around 3
// lines before/after the change.
func diffToPrettyString(diffs []diffmatchpatch.Diff) string {
	var buff bytes.Buffer
	for index, diff := range diffs {
		text := diff.Text

		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			buff.WriteString(deploy.OpCreate.Color())
			buff.WriteString(text)
			buff.WriteString(colors.Reset)
		case diffmatchpatch.DiffDelete:
			buff.WriteString(deploy.OpDelete.Color())
			buff.WriteString(text)
			buff.WriteString(colors.Reset)
		case diffmatchpatch.DiffEqual:
			lines := strings.SplitAfter(text, "\n")
			var trimmedLines []string
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					trimmedLines = append(trimmedLines, line)
				}
			}

			lines = trimmedLines

			// Show the unchanged text in white.
			buff.WriteString(colors.Reset)
			if index == 0 {
				// First chunk of the file.
				if len(lines) > 4 {
					buff.WriteString("...\n")
					buff.WriteString(lines[len(lines)-3])
					buff.WriteString(lines[len(lines)-2])
					buff.WriteString(lines[len(lines)-1])
				} else {
					buff.WriteString(text)
				}
			} else if index == len(diffs)-1 {
				if len(lines) > 4 {
					buff.WriteString(lines[0])
					buff.WriteString(lines[1])
					buff.WriteString(lines[2])
					buff.WriteString("...\n")
				} else {
					buff.WriteString(text)
				}
			} else {
				if len(lines) > 7 {
					buff.WriteString(lines[0])
					buff.WriteString(lines[1])
					buff.WriteString(lines[2])
					buff.WriteString("...\n")
					buff.WriteString(lines[len(lines)-3])
					buff.WriteString(lines[len(lines)-2])
					buff.WriteString(lines[len(lines)-1])
				} else {
					buff.WriteString(text)
				}
			}
		}
	}

	buff.WriteString(deploy.OpUpdate.Color())

	return buff.String()
}

func addedIndentString(currentIndent string) string {
	return indentStringWithPrefix(currentIndent, "+ ")
}
func deletedIndentString(currentIndent string) string {
	return indentStringWithPrefix(currentIndent, "- ")
}
func changedIndentString(currentIndent string) string {
	return indentStringWithPrefix(currentIndent, "~ ")
}
func unchangedIndentString(currentIndent string) string {
	return indentStringWithPrefix(currentIndent, "= ")
}

func indentStringWithPrefix(currentIndent string, prefix string) string {
	contract.Assert(len(prefix) == 2)
	return currentIndent[:len(currentIndent)-2] + prefix
}

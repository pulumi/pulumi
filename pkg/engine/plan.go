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
	Seen    map[resource.URN]deploy.Step
	Shown   map[resource.URN]bool
}

func newPreviewActions(opts deployOptions) *previewActions {
	return &previewActions{
		Ops:   make(map[deploy.StepOp]int),
		Opts:  opts,
		Seen:  make(map[resource.URN]deploy.Step),
		Shown: make(map[resource.URN]bool),
	}
}

func (acts *previewActions) Run(step deploy.Step) (resource.Status, error) {
	// Print this step information (resource and all its properties).
	if shouldShow(acts.Seen, step, acts.Opts) {
		printStep(&acts.Summary, step, acts.Seen, acts.Shown, acts.Opts.Summary, acts.Opts.Detailed, true, 0 /*indent*/)
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

	actions := newPreviewActions(result.Options)
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
func shouldShow(seen map[resource.URN]deploy.Step, step deploy.Step, opts deployOptions) bool {
	// Ensure we've marked this step as observed.
	seen[step.URN()] = step

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
			// 4 spaces, plus 2 for "+ ", "- ", and " " leaders
			b.WriteString(fmt.Sprintf("      %v: %v\n", key, config[tokens.ModuleMember(key)]))
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

	if changes > 0 || counts[deploy.OpSame] > 0 {
		kind += ":"
	}

	b.WriteString(fmt.Sprintf("%vinfo%v: %v %v %v\n",
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

func printStep(b *bytes.Buffer, step deploy.Step, seen map[resource.URN]deploy.Step, shown map[resource.URN]bool,
	summary bool, detailed bool, planning bool, indent int) {
	op := step.Op()

	// First, indent to the same level as this resource has parents, and toggle the level of detail accordingly.
	// TODO[pulumi/pulumi#340]: this isn't entirely correct.  Conventionally, all children are created adjacent to
	//     their parents, so this often does the right thing, but not always.  For instance, we can have interleaved
	//     infrastructure that gets emitted in the middle of the flow, making things look like they are parented
	//     incorrectly.  The real solution here is to have a more first class way of structuring the output.
	for p := step.Res().Parent; p != ""; {
		par := seen[p]
		if par == nil {
			// This can happen during deletes, since we delete children before parents.
			// TODO[pulumi/pulumi#340]: we need to figure out how best to display this sequence; at the very
			//     least, it would be ideal to preserve the indentation.
			break
		}
		if !shown[p] {
			// If the parent isn't yet shown, print it now as a summary.
			printStep(b, par, seen, shown, true, false, planning, indent)
		}
		indent++
		p = par.Res().Parent
	}

	// Print the indentation.
	b.WriteString(getIndentationString(indent, op))

	// Next, print the resource type (since it is easy on the eyes and can be quickly identified).
	printStepHeader(b, step)

	// Next print the resource URN, properties, etc.
	var replaces []resource.PropertyKey
	if step.Op() == deploy.OpCreateReplacement {
		replaces = step.(*deploy.CreateStep).Keys()
	} else if step.Op() == deploy.OpReplace {
		replaces = step.(*deploy.ReplaceStep).Keys()
	}
	printResourceProperties(b, step.URN(), step.Old(), step.New(), replaces, summary, detailed, planning, indent, op)

	// Reset the color and mark this as shown -- we're done.
	b.WriteString(colors.Reset)
	shown[step.URN()] = true
}

func printStepHeader(b *bytes.Buffer, step deploy.Step) {
	b.WriteString(fmt.Sprintf("%s: (%s)\n", string(step.Type()), step.Op()))
}

func getIndentationString(indent int, op deploy.StepOp) string {
	result := ""

	for i := 0; i < indent; i++ {
		result += "    "
	}

	if result == "" {
		return result
	}

	switch op {
	case deploy.OpSame:
		return result
	case deploy.OpCreate:
		return addedIndentString(result)
	case deploy.OpUpdate:
	case deploy.OpReplace:
	case deploy.OpCreateReplacement:
	case deploy.OpDeleteReplaced:
		return changedIndentString(result)
	case deploy.OpDelete:
		return deletedIndentString(result)
	default:
		contract.Assertf(false, "Switch case not handled: %v", op)
	}

	return result
}

func writeWithSpecificIndentAndPrefix(
	b *bytes.Buffer, indent int,
	colorOp deploy.StepOp, prefixOp deploy.StepOp,
	format string, a ...interface{}) {

	b.WriteString(colors.Reset)
	b.WriteString(colorOp.Color())
	b.WriteString(getIndentationString(indent, prefixOp))
	b.WriteString(fmt.Sprintf(format, a...))
	b.WriteString(colors.Reset)
}

func writeWithIndent(b *bytes.Buffer, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithSpecificIndentAndPrefix(b, indent, op, op, format, a...)
}

func writeWithIndentAndNoPrefix(b *bytes.Buffer, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithSpecificIndentAndPrefix(b, indent, op, deploy.OpSame, format, a...)
}

func write(b *bytes.Buffer, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, 0, op, format, a...)
}

func writeVerbatim(b *bytes.Buffer, op deploy.StepOp, value string) {
	writeVerbatimWithIndent(b, 0, op, value)
}

func writeVerbatimWithIndent(b *bytes.Buffer, indent int, op deploy.StepOp, value string) {
	writeWithIndent(b, indent, op, "%s", value)
}

func printResourceProperties(
	b *bytes.Buffer, urn resource.URN, old *resource.State, new *resource.State,
	replaces []resource.PropertyKey, summary bool, detailed bool, planning bool, indent int, op deploy.StepOp) {

	indent++

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID

	// For these simple properties, print them as 'same' if they're just an update or replace.
	var simplePropOp = considerSameIfNotCreateOrDelete(op)

	if old != nil {
		id = old.ID
	}

	if id != "" {
		writeWithIndent(b, indent, simplePropOp, "[id=%s]\n", string(id))
	}

	if urn != "" {
		writeWithIndent(b, indent, simplePropOp, "[urn=%s]\n", urn)
	}

	if !summary {
		// Print all of the properties associated with this resource.
		if old == nil && new != nil {
			printObject(b, new.AllInputs(), planning, indent, op)
		} else if new == nil && old != nil {
			printObject(b, old.AllInputs(), planning, indent, op)
		} else {
			printOldNewDiffs(b, old.AllInputs(), new.AllInputs(), replaces, detailed, planning, indent, op)
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

func printObject(
	b *bytes.Buffer, props resource.PropertyMap, planning bool,
	indent int, op deploy.StepOp) {

	// Compute the maximum with of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v, planning) {
			printPropertyTitle(b, string(k), maxkey, indent, op)
			printPropertyValue(b, v, planning, indent, op)
		}
	}
}

// printResourceOutputProperties prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func printResourceOutputProperties(b *bytes.Buffer, step deploy.Step, indent int) {
	indent++
	op := step.Op()

	// Only certain kinds of steps have output properties associated with them.
	if op != deploy.OpCreate &&
		op != deploy.OpCreateReplacement &&
		op != deploy.OpUpdate {
		return
	}

	op = considerSameIfNotCreateOrDelete(op)

	// First fetch all the relevant property maps that we may consult.
	newins := step.New().Inputs
	newouts := step.New().Outputs
	var oldouts resource.PropertyMap
	if old := step.Old(); old != nil {
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
					writeWithIndent(b, indent, op, "---outputs:---\n")
					firstout = false
				}
				printPropertyTitle(b, string(k), maxkey, indent, op)
				printPropertyValue(b, newout, false, indent, op)
			}
		}
	}
}

func considerSameIfNotCreateOrDelete(op deploy.StepOp) deploy.StepOp {
	if op == deploy.OpCreate || op == deploy.OpDelete || op == deploy.OpDeleteReplaced {
		return op
	}

	return deploy.OpSame
}

func shouldPrintPropertyValue(v resource.PropertyValue, outs bool) bool {
	if v.IsNull() {
		return false // don't print nulls (they just clutter up the output).
	}
	if v.IsString() && v.StringValue() == "" {
		return false // don't print empty strings either.
	}
	if v.IsArray() && len(v.ArrayValue()) == 0 {
		return false // skip empty arrays, since they are often uninteresting default values.
	}
	if v.IsObject() && len(v.ObjectValue()) == 0 {
		return false // skip objects with no properties, since they are also uninteresting.
	}
	if v.IsObject() && len(v.ObjectValue()) == 0 {
		return false // skip objects with no properties, since they are also uninteresting.
	}
	if v.IsOutput() && !outs {
		// also don't show output properties until the outs parameter tells us to.
		return false
	}
	return true
}

func printPropertyTitle(b *bytes.Buffer, name string, align int, indent int, op deploy.StepOp) {
	writeWithIndent(b, indent, op, "%-"+strconv.Itoa(align)+"s: ", name)
}

func printPropertyValue(
	b *bytes.Buffer, v resource.PropertyValue, planning bool,
	indent int, op deploy.StepOp) {

	if v.IsNull() {
		writeVerbatim(b, op, "<null>")
	} else if v.IsBool() {
		write(b, op, "%t", v.BoolValue())
	} else if v.IsNumber() {
		write(b, op, "%v", v.NumberValue())
	} else if v.IsString() {
		write(b, op, "%q", v.StringValue())
	} else if v.IsArray() {
		arr := v.ArrayValue()
		if len(arr) == 0 {
			writeVerbatim(b, op, "[]")
		} else {
			writeVerbatim(b, op, "[\n")
			for i, elem := range arr {
				writeWithIndent(b, indent, op, "    [%d]: ", i)
				printPropertyValue(b, elem, planning, indent+1, op)
			}
			writeWithIndent(b, indent, op, "]")
		}
	} else if v.IsAsset() {
		a := v.AssetValue()
		if text, has := a.GetText(); has {
			write(b, op, "asset(text:%s) {\n", shortHash(a.Hash))

			massaged := massageText(text)

			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(massaged, "\n")
			for _, line := range lines {
				writeWithIndentAndNoPrefix(b, indent, op, "    %s\n", line)
			}
			writeWithIndent(b, indent, op, "}")
		} else if path, has := a.GetPath(); has {
			write(b, op, "asset(file:%s) { %s }", shortHash(a.Hash), path)
		} else {
			contract.Assert(a.IsURI())
			write(b, op, "asset(uri:%s) { %s }", shortHash(a.Hash), a.URI)
		}
	} else if v.IsArchive() {
		a := v.ArchiveValue()
		if assets, has := a.GetAssets(); has {
			write(b, op, "archive(assets:%s) {\n", shortHash(a.Hash))
			var names []string
			for name := range assets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				printAssetOrArchive(b, assets[name], name, planning, indent, op)
			}
			writeWithIndent(b, indent, op, "}")
		} else if path, has := a.GetPath(); has {
			write(b, op, "archive(file:%s) { %s }", shortHash(a.Hash), path)
		} else {
			contract.Assert(a.IsURI())
			write(b, op, "archive(uri:%s) { %v }", shortHash(a.Hash), a.URI)
		}
	} else if v.IsComputed() || v.IsOutput() {
		writeVerbatim(b, op, v.TypeString())
	} else {
		contract.Assert(v.IsObject())
		obj := v.ObjectValue()
		if len(obj) == 0 {
			writeVerbatim(b, op, "{}")
		} else {
			writeVerbatim(b, op, "{\n")
			printObject(b, obj, planning, indent+1, op)
			writeWithIndent(b, indent, op, "}")
		}
	}
	writeVerbatim(b, op, "\n")
}

func printAssetOrArchive(
	b *bytes.Buffer, v interface{}, name string, planning bool,
	indent int, op deploy.StepOp) {

	writeWithIndent(b, indent, op, "    \"%v\": ", name)
	printPropertyValue(b, assetOrArchiveToPropertyValue(v), planning, indent+1, op)
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

func printOldNewDiffs(
	b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap,
	replaces []resource.PropertyKey, detailed bool, planning bool, indent int, op deploy.StepOp) {

	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, detailed, replaces, false, planning, indent)
	} else {
		printObject(b, news, planning, indent, op)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff, detailed bool,
	replaces []resource.PropertyKey, causedReplace bool, planning bool, indent int) {

	contract.Assert(indent > 0)

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
		title := func(_op deploy.StepOp) {
			printPropertyTitle(b, string(k), maxkey, indent, _op)
		}
		if add, isadd := diff.Adds[k]; isadd {
			if shouldPrintPropertyValue(add, planning) {
				printAdd(b, add, title, true, planning, indent)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				printDelete(b, delete, title, true, planning, indent)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}
			printPropertyValueDiff(b, title, update, detailed, causedReplace, planning, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same, planning) {
			title(deploy.OpSame)
			printPropertyValue(b, diff.Sames[k], planning, indent, deploy.OpSame)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, title func(deploy.StepOp), diff resource.ValueDiff, detailed bool,
	causedReplace bool, planning bool, indent int) {

	op := deploy.OpUpdate
	contract.Assert(indent > 0)
	// contract.Assert(len(indent) > 2)

	if diff.Array != nil {
		title(op)
		writeVerbatim(b, op, "[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			newIndent := indent + 2

			titleFunc := func(_op deploy.StepOp) {
				writeWithIndent(b, indent+1, _op, "[%d]: ", i)
			}
			if add, isadd := a.Adds[i]; isadd {
				printAdd(b, add, titleFunc, true, planning, newIndent)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				printDelete(b, delete, titleFunc, true, planning, newIndent)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(b, titleFunc, update, detailed, causedReplace, planning, indent)
			} else {
				titleFunc(deploy.OpSame)
				printPropertyValue(b, a.Sames[i], planning, newIndent, deploy.OpSame)
			}
		}
		writeWithIndent(b, indent, op, "]\n")
	} else if diff.Object != nil {
		title(op)
		writeVerbatim(b, op, "{\n")
		printObjectDiff(b, *diff.Object, detailed, nil, causedReplace, planning, indent+1)
		writeWithIndent(b, indent, op, "}\n")
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
			printDelete(b, diff.Old, title, causedReplace, planning, indent)
		}
		if shouldPrintNew {
			printAdd(b, diff.New, title, causedReplace, planning, indent)
		}
	}
}

func printDelete(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp),
	causedReplace bool, planning bool, indent int) {

	var op deploy.StepOp
	if causedReplace {
		op = deploy.OpDelete
	} else {
		op = deploy.OpUpdate
	}

	title(op)
	printPropertyValue(b, v, planning, indent, op)
}

func printAdd(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp),
	causedReplace bool, planning bool, indent int) {

	var op deploy.StepOp
	if causedReplace {
		op = deploy.OpCreate
	} else {
		op = deploy.OpUpdate
	}

	title(op)
	printPropertyValue(b, v, planning, indent, op)
}

func printArchiveDiff(
	b *bytes.Buffer, title func(deploy.StepOp),
	oldArchive *resource.Archive, newArchive *resource.Archive,
	planning bool, indent int) {

	// TODO: this could be called recursively from itself.  In the recursive case, we might have an
	// archive that actually hasn't changed.  Check for that, and terminate the diff printing.

	op := deploy.OpUpdate

	hashChange := getTextChangeString(shortHash(oldArchive.Hash), shortHash(newArchive.Hash))

	if oldPath, has := oldArchive.GetPath(); has {
		if newPath, has := newArchive.GetPath(); has {
			title(op)
			write(b, op, "archive(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else if oldURI, has := oldArchive.GetURI(); has {
		if newURI, has := newArchive.GetURI(); has {
			title(op)
			write(b, op, "archive(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	} else {
		contract.Assert(oldArchive.IsAssets())
		oldAssets, _ := oldArchive.GetAssets()

		if newAssets, has := newArchive.GetAssets(); has {
			title(op)
			write(b, op, "archive(assets:%s) {\n", hashChange)
			printAssetsDiff(b, oldAssets, newAssets, planning, indent+1)
			writeWithIndent(b, indent, deploy.OpUpdate, "}\n")
			return
		}
	}

	// Type of archive changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldArchive),
		title, false /*causedReplace*/, planning, indent)
	printAdd(
		b, assetOrArchiveToPropertyValue(newArchive),
		title, false /*causedReplace*/, planning, indent)
}

func printAssetsDiff(
	b *bytes.Buffer,
	oldAssets map[string]interface{}, newAssets map[string]interface{},
	planning bool, indent int) {

	// Diffing assets proceeds by getting the sorted list of asset names from both the old and
	// new assets, and then stepwise processing each.  For any asset in old that isn't in new,
	// we print this out as a delete.  For any asset in new that isn't in old, we print this out
	// as an add.  For any asset in both we print out of it is unchanged or not.  If so, we
	// recurse on that data to print out how it changed.

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
		deleteOld := false
		addNew := false
		if i < len(oldNames) && j < len(newNames) {
			oldName := oldNames[i]
			newName := newNames[j]

			if oldName == newName {
				title := func(_op deploy.StepOp) {
					printPropertyTitle(b, "\""+oldName+"\"", maxkey, indent, _op)
				}

				oldAsset := oldAssets[oldName]
				newAsset := newAssets[newName]

				switch t := oldAsset.(type) {
				case *resource.Archive:
					printArchiveDiff(b, title, t, newAsset.(*resource.Archive), planning, indent)
				case *resource.Asset:
					printAssetDiff(b, title, t, newAsset.(*resource.Asset), planning, indent)
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

		newIndent := indent + 1
		if deleteOld {
			oldName := oldNames[i]
			title := func(_op deploy.StepOp) {
				printPropertyTitle(b, "\""+oldName+"\"", maxkey, indent, _op)
			}
			printDelete(b, assetOrArchiveToPropertyValue(oldAssets[oldName]), title, false, planning, newIndent)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			title := func(_op deploy.StepOp) {
				printPropertyTitle(b, "\""+newName+"\"", maxkey, indent, _op)
			}
			printAdd(b, assetOrArchiveToPropertyValue(newAssets[newName]), title, false, planning, newIndent)
			j++
		}
	}
}

func makeAssetHeader(asset *resource.Asset) string {
	var assetType string
	var contents string

	if path, has := asset.GetPath(); has {
		assetType = "file"
		contents = path
	} else if uri, has := asset.GetURI(); has {
		assetType = "uri"
		contents = uri
	} else {
		assetType = "text"
		contents = "..."
	}

	return fmt.Sprintf("asset(%s:%s) { %s }\n", assetType, shortHash(asset.Hash), contents)
}

func printAssetDiff(
	b *bytes.Buffer, title func(deploy.StepOp),
	oldAsset *resource.Asset, newAsset *resource.Asset,
	planning bool, indent int) {

	op := deploy.OpUpdate

	// If the assets aren't changed, just print out: = assetName: type(hash)
	if oldAsset.Hash == newAsset.Hash {
		op = deploy.OpSame
		title(op)
		write(b, op, makeAssetHeader(oldAsset))
		return
	}

	// if the asset changed, print out: ~ assetName: type(hash->hash) details...

	hashChange := getTextChangeString(shortHash(oldAsset.Hash), shortHash(newAsset.Hash))

	if oldText, has := oldAsset.GetText(); has {
		if newText, has := newAsset.GetText(); has {
			title(deploy.OpUpdate)
			write(b, op, "asset(text:%s) {\n", hashChange)

			massagedOldText := massageText(oldText)
			massagedNewText := massageText(newText)

			differ := diffmatchpatch.New()
			differ.DiffTimeout = 0

			hashed1, hashed2, lineArray := differ.DiffLinesToChars(massagedOldText, massagedNewText)
			diffs1 := differ.DiffMain(hashed1, hashed2, false)
			diffs2 := differ.DiffCharsToLines(diffs1, lineArray)

			b.WriteString(diffToPrettyString(diffs2, indent+1))

			writeWithIndent(b, indent, op, "}\n")
			return
		}
	} else if oldPath, has := oldAsset.GetPath(); has {
		if newPath, has := newAsset.GetPath(); has {
			title(deploy.OpUpdate)
			write(b, op, "asset(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else {
		contract.Assert(oldAsset.IsURI())

		oldURI, _ := oldAsset.GetURI()
		if newURI, has := newAsset.GetURI(); has {
			title(deploy.OpUpdate)
			write(b, op, "asset(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	}

	// Type of asset changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldAsset),
		title, false /*causedReplace*/, planning, indent)
	printAdd(
		b, assetOrArchiveToPropertyValue(newAsset),
		title, false /*causedReplace*/, planning, indent)
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
//
// TODO(https://github.com/pulumi/pulumi/issues/592) this is baking in a lot of knowledge about
// pulumi serialized functions.  We should try to move to an alternative mode that isn't so brittle.
// Options include:
//   1. Have a documented delimeter format that plan.go will look for.  Have the function serializer
//      emit those delimeters around code that should be ignored.
//   2. Have our resource generation code supply not just the resource, but the "user presentable"
//      resource that cuts out a lot of cruft.  We could then just diff that content here.
func massageText(text string) string {
	shaRegexp, _ := regexp.Compile("__[a-zA-Z0-9]{40}")
	closureRegexp, _ := regexp.Compile(`    with\(\{ .* \}\) \{`)

	// Only do this for strings that match our serialized function pattern.
	if !shaRegexp.MatchString(text) || !closureRegexp.MatchString(text) {
		return text
	}

	for {
		newText := strings.Replace(text, "\n\n\n", "\n\n", -1)
		if len(newText) == len(text) {
			break
		}

		text = newText
	}

	text = shaRegexp.ReplaceAllString(text, "__shaHash")
	text = closureRegexp.ReplaceAllString(text, "    with (__closure) {")

	return text
}

// diffToPrettyString takes the full diff produed by diffmatchpatch and condenses it into something
// useful we can print to the console.  Specifically, while it includes any adds/removes in
// green/red, it will also show portions of the unchanged text to help give surrounding context to
// those add/removes. Because the unchanged portions may be very large, it only included around 3
// lines before/after the change.
func diffToPrettyString(diffs []diffmatchpatch.Diff, indent int) string {
	var buff bytes.Buffer

	writeDiff := func(op deploy.StepOp, text string) {
		writeWithIndentAndNoPrefix(&buff, indent, op, "%s", text)
	}

	for index, diff := range diffs {
		text := diff.Text

		lines := strings.Split(text, "\n")

		printLines := func(op deploy.StepOp, startInclusive int, endExclusive int) {
			for i := startInclusive; i < endExclusive; i++ {
				writeDiff(op, lines[i])
				buff.WriteString("\n")
			}
		}

		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			printLines(deploy.OpCreate, 0, len(lines))
		case diffmatchpatch.DiffDelete:
			printLines(deploy.OpDelete, 0, len(lines))
		case diffmatchpatch.DiffEqual:
			var trimmedLines []string
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					trimmedLines = append(trimmedLines, line)
				}
			}

			lines = trimmedLines

			// Show the unchanged text in white.

			if index == 0 {
				// First chunk of the file.
				if len(lines) > 4 {
					writeDiff(deploy.OpSame, "...\n")
					printLines(deploy.OpSame, len(lines)-3, len(lines))
					continue
				}
			} else if index == len(diffs)-1 {
				if len(lines) > 4 {
					printLines(deploy.OpSame, 0, 3)
					writeDiff(deploy.OpSame, "...\n")
					continue
				}
			} else {
				if len(lines) > 7 {
					printLines(deploy.OpSame, 0, 3)
					writeDiff(deploy.OpSame, "...\n")
					printLines(deploy.OpSame, len(lines)-3, len(lines))
					continue
				}
			}

			printLines(deploy.OpSame, 0, len(lines))
		}
	}

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

func indentStringWithPrefix(currentIndent string, prefix string) string {
	contract.Assert(len(prefix) == 2)
	return currentIndent[:len(currentIndent)-2] + prefix
}

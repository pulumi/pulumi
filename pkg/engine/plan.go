// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
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

	// Create a context for plugins.
	ctx, err := plugin.NewContext(opts.Diag, nil, pwd, info.TracingSpan)
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

// Chdir changes the directory so that all operations from now on are relative to the project we are working with.
// It returns a function that, when run, restores the old working directory.
func (res *planResult) Chdir() (func(), error) {
	if res.Ctx.Pwd == "" {
		return func() {}, nil
	}
	oldpwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err = os.Chdir(res.Ctx.Pwd); err != nil {
		return nil, errors.Wrapf(err, "could not change to the project working directory")
	}
	return func() {
		// Restore the working directory after planning completes.
		cderr := os.Chdir(oldpwd)
		contract.IgnoreError(cderr)
	}, nil
}

// Walk enumerates all steps in the plan, calling out to the provided action at each step.  It returns four things: the
// resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step that
// failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (res *planResult) Walk(events deploy.Events, preview bool) (deploy.PlanSummary,
	deploy.Step, resource.Status, error) {
	opts := deploy.Options{
		Events:   events,
		Parallel: res.Options.Parallel,
	}

	// Fetch a plan iterator and keep walking it until we are done.
	iter, err := res.Plan.Start(opts)
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}

	step, err := iter.Next()
	if err != nil {
		closeerr := iter.Close() // ignore close errors; the Next error trumps
		contract.IgnoreError(closeerr)
		return nil, nil, resource.StatusOK, err
	}

	for step != nil {
		// Perform any per-step actions.
		rst, err := iter.Apply(step, preview)

		// If an error occurred, exit early.
		if err != nil {
			closeerr := iter.Close() // ignore close errors; the action error trumps
			contract.IgnoreError(closeerr)
			return iter, step, rst, err
		}
		contract.Assert(rst == resource.StatusOK)

		step, err = iter.Next()
		if err != nil {
			closeerr := iter.Close() // ignore close errors; the action error trumps
			contract.IgnoreError(closeerr)
			return iter, step, resource.StatusOK, err
		}
	}

	// Finally, return a summary and the resulting plan information.
	return iter, nil, resource.StatusOK, iter.Close()
}

func (res *planResult) Close() error {
	return res.Ctx.Close()
}

func (eng *Engine) printPlan(result *planResult) error {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, true)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPreviewing changes:%v\n", colors.SpecUnimportant, colors.Reset))
	result.Options.Events <- stdOutEventWithColor(&prelude, result.Options.Color)

	actions := newPreviewActions(result.Options)
	_, _, _, err := result.Walk(actions, true)
	if err != nil {
		return errors.Errorf("An error occurred while advancing the preview: %v", err)
	}

	if !result.Options.Diag.Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return errors.New("One or more errors occurred during this preview")
	}

	// Print a summary of operation counts.
	var summary bytes.Buffer
	printChangeSummary(&summary, actions.Ops, true)
	result.Options.Events <- stdOutEventWithColor(&summary, result.Options.Color)
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

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step deploy.Step) bool {
	return step.URN().Type() == resource.RootStackType
}

func printPrelude(b *bytes.Buffer, result *planResult, planning bool) {
	// If there are configuration variables, show them.
	if result.Options.ShowConfig {
		printConfig(b, result.Info.Target.Config)
	}
}

func printConfig(b *bytes.Buffer, cfg config.Map) {
	b.WriteString(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset))
	if cfg != nil {
		var keys []string
		for key := range cfg {
			keys = append(keys, string(key))
		}
		sort.Strings(keys)
		for _, key := range keys {
			v, err := cfg[tokens.ModuleMember(key)].Value(config.NewBlindingDecrypter())
			contract.Assert(err == nil)
			b.WriteString(fmt.Sprintf("    %v: %v\n", key, v))
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

// stepParentIndent computes a step's parent indentation.  If print is true, it also prints parents as it goes.
func stepParentIndent(b *bytes.Buffer, step deploy.Step,
	seen map[resource.URN]deploy.Step, shown map[resource.URN]bool, planning bool, indent int, print bool) int {
	for p := step.Res().Parent; p != ""; {
		par := seen[p]
		if par == nil {
			// This can happen during deletes, since we delete children before parents.
			// TODO[pulumi/pulumi#340]: we need to figure out how best to display this sequence; at the very
			//     least, it would be ideal to preserve the indentation.
			break
		}
		if print && !shown[p] {
			// If the parent isn't yet shown, print it now as a summary.
			printStep(b, par, seen, shown, true, false, planning, indent)
		}
		indent++
		p = par.Res().Parent
	}
	return indent
}

func printStep(b *bytes.Buffer, step deploy.Step, seen map[resource.URN]deploy.Step, shown map[resource.URN]bool,
	summary bool, detailed bool, planning bool, indent int) {
	op := step.Op()

	// First, indent to the same level as this resource has parents, and toggle the level of detail accordingly.
	// TODO[pulumi/pulumi#340]: this isn't entirely correct.  Conventionally, all children are created adjacent to
	//     their parents, so this often does the right thing, but not always.  For instance, we can have interleaved
	//     infrastructure that gets emitted in the middle of the flow, making things look like they are parented
	//     incorrectly.  The real solution here is to have a more first class way of structuring the output.
	indent = stepParentIndent(b, step, seen, shown, planning, indent, true)

	// Print the indentation.
	b.WriteString(getIndentationString(indent, op, false))

	// First, print out the operation's prefix.
	b.WriteString(op.Prefix())

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

func getIndentationString(indent int, op deploy.StepOp, prefix bool) string {
	var result string
	for i := 0; i < indent; i++ {
		result += "    "
	}

	if result == "" {
		contract.Assertf(!prefix, "Expected indention for a prefixed line")
		return result
	}

	var rp string
	if prefix {
		rp = op.RawPrefix()
	} else {
		rp = "  "
	}
	contract.Assert(len(rp) == 2)
	contract.Assert(len(result) >= 2)
	return result[:len(result)-2] + rp
}

func writeWithIndent(b *bytes.Buffer, indent int, op deploy.StepOp, prefix bool, format string, a ...interface{}) {
	b.WriteString(op.Color())
	b.WriteString(getIndentationString(indent, op, prefix))
	b.WriteString(fmt.Sprintf(format, a...))
	b.WriteString(colors.Reset)
}

func writeWithIndentPrefix(b *bytes.Buffer, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, indent, op, true, format, a...)
}

func writeWithIndentNoPrefix(b *bytes.Buffer, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, indent, op, false, format, a...)
}

func write(b *bytes.Buffer, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndentNoPrefix(b, 0, op, format, a...)
}

func writeVerbatim(b *bytes.Buffer, op deploy.StepOp, value string) {
	writeWithIndentNoPrefix(b, 0, op, "%s", value)
}

func printResourceProperties(
	b *bytes.Buffer, urn resource.URN, old *resource.State, new *resource.State,
	replaces []resource.PropertyKey, summary bool, detailed bool, planning bool, indent int, op deploy.StepOp) {

	indent++

	// For these simple properties, print them as 'same' if they're just an update or replace.
	simplePropOp := considerSameIfNotCreateOrDelete(op)

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	if old != nil {
		id = old.ID
	}

	// Always print the ID and URN.
	if id != "" {
		writeWithIndentNoPrefix(b, indent, simplePropOp, "[id=%s]\n", string(id))
	}
	if urn != "" {
		writeWithIndentNoPrefix(b, indent, simplePropOp, "[urn=%s]\n", urn)
	}

	// If not summarizing, print all of the properties associated with this resource.
	if !summary {
		if old == nil && new != nil {
			printObject(b, new.Inputs, planning, indent, op, false)
		} else if new == nil && old != nil {
			printObject(b, old.Inputs, planning, indent, op, false)
		} else {
			printOldNewDiffs(b, old.Inputs, new.Inputs, replaces, detailed, planning, indent, op)
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
	indent int, op deploy.StepOp, prefix bool) {

	// Compute the maximum with of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v, planning) {
			printPropertyTitle(b, string(k), maxkey, indent, op, prefix)
			printPropertyValue(b, v, planning, indent, op, prefix)
		}
	}
}

// printResourceOutputProperties prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func printResourceOutputProperties(b *bytes.Buffer, step deploy.Step,
	seen map[resource.URN]deploy.Step, shown map[resource.URN]bool, indent int) {
	// Only certain kinds of steps have output properties associated with them.
	new := step.New()
	if new == nil || new.Outputs == nil {
		return
	}
	op := considerSameIfNotCreateOrDelete(step.Op())

	// Now compute the indentation level, in part based on the parents.
	indent++ // indent for the resource.
	indent = stepParentIndent(b, step, seen, shown, false, indent, false)

	// First fetch all the relevant property maps that we may consult.
	ins := new.Inputs
	outs := new.Outputs

	// Now sort the keys and enumerate each output property in a deterministic order.
	firstout := true
	keys := outs.StableKeys()
	maxkey := maxKey(keys)
	for _, k := range keys {
		out := outs[k]
		// Print this property if it is printable and either ins doesn't have it or it's different.
		if shouldPrintPropertyValue(out, true) {
			var print bool
			if in, has := ins[k]; has {
				print = (out.Diff(in) != nil)
			} else {
				print = true
			}

			if print {
				if firstout {
					writeWithIndentNoPrefix(b, indent, op, "---outputs:---\n")
					firstout = false
				}
				printPropertyTitle(b, string(k), maxkey, indent, op, false)
				printPropertyValue(b, out, false, indent, op, false)
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

func printPropertyTitle(b *bytes.Buffer, name string, align int, indent int, op deploy.StepOp, prefix bool) {
	writeWithIndent(b, indent, op, prefix, "%-"+strconv.Itoa(align)+"s: ", name)
}

func printPropertyValue(
	b *bytes.Buffer, v resource.PropertyValue, planning bool,
	indent int, op deploy.StepOp, prefix bool) {

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
				writeWithIndent(b, indent, op, prefix, "    [%d]: ", i)
				printPropertyValue(b, elem, planning, indent+1, op, prefix)
			}
			writeWithIndent(b, indent, op, prefix, "]")
		}
	} else if v.IsAsset() {
		a := v.AssetValue()
		if text, has := a.GetText(); has {
			write(b, op, "asset(text:%s) {\n", shortHash(a.Hash))

			massaged := massageText(text)

			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(massaged, "\n")
			for _, line := range lines {
				writeWithIndentNoPrefix(b, indent, op, "    %s\n", line)
			}
			writeWithIndent(b, indent, op, prefix, "}")
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
				printAssetOrArchive(b, assets[name], name, planning, indent, op, prefix)
			}
			writeWithIndent(b, indent, op, prefix, "}")
		} else if path, has := a.GetPath(); has {
			write(b, op, "archive(file:%s) { %s }", shortHash(a.Hash), path)
		} else {
			contract.Assert(a.IsURI())
			write(b, op, "archive(uri:%s) { %v }", shortHash(a.Hash), a.URI)
		}
	} else if v.IsComputed() || v.IsOutput() {
		// We render computed and output values differently depending on whether or not we are planning or deploying:
		// in the former case, we display `computed<type>` or `output<type>`; in the former we display `undefined`.
		// This is because we currently cannot distinguish between user-supplied undefined values and input properties
		// that are undefined because they were sourced from undefined values in other resources' output properties.
		// Once we have richer information about the dataflow between resources, we should be able to do a better job
		// here (pulumi/pulumi#234).
		if planning {
			writeVerbatim(b, op, v.TypeString())
		} else {
			write(b, op, "undefined")
		}
	} else {
		contract.Assert(v.IsObject())
		obj := v.ObjectValue()
		if len(obj) == 0 {
			writeVerbatim(b, op, "{}")
		} else {
			writeVerbatim(b, op, "{\n")
			printObject(b, obj, planning, indent+1, op, prefix)
			writeWithIndent(b, indent, op, prefix, "}")
		}
	}
	writeVerbatim(b, op, "\n")
}

func printAssetOrArchive(
	b *bytes.Buffer, v interface{}, name string, planning bool,
	indent int, op deploy.StepOp, prefix bool) {
	writeWithIndent(b, indent, op, prefix, "    \"%v\": ", name)
	printPropertyValue(b, assetOrArchiveToPropertyValue(v), planning, indent+1, op, prefix)
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
		printObject(b, news, planning, indent, op, true)
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
		titleFunc := func(top deploy.StepOp, prefix bool) {
			printPropertyTitle(b, string(k), maxkey, indent, top, prefix)
		}
		if add, isadd := diff.Adds[k]; isadd {
			if shouldPrintPropertyValue(add, planning) {
				printAdd(b, add, titleFunc, true, planning, indent)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				printDelete(b, delete, titleFunc, true, planning, indent)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}
			printPropertyValueDiff(b, titleFunc, update, detailed, causedReplace, planning, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same, planning) {
			titleFunc(deploy.OpSame, false)
			printPropertyValue(b, diff.Sames[k], planning, indent, deploy.OpSame, false)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	diff resource.ValueDiff, detailed bool, causedReplace bool, planning bool, indent int) {

	op := deploy.OpUpdate
	contract.Assert(indent > 0)

	if diff.Array != nil {
		titleFunc(op, true)
		writeVerbatim(b, op, "[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			elemTitleFunc := func(eop deploy.StepOp, eprefix bool) {
				writeWithIndent(b, indent+1, eop, eprefix, "[%d]: ", i)
			}
			if add, isadd := a.Adds[i]; isadd {
				printAdd(b, add, elemTitleFunc, true, planning, indent+2)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				printDelete(b, delete, elemTitleFunc, true, planning, indent+2)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(b, elemTitleFunc, update, detailed, causedReplace, planning, indent+2)
			} else {
				elemTitleFunc(deploy.OpSame, false)
				printPropertyValue(b, a.Sames[i], planning, indent+2, deploy.OpSame, false)
			}
		}
		writeWithIndentNoPrefix(b, indent, op, "]\n")
	} else if diff.Object != nil {
		titleFunc(op, true)
		writeVerbatim(b, op, "{\n")
		printObjectDiff(b, *diff.Object, detailed, nil, causedReplace, planning, indent+1)
		writeWithIndentNoPrefix(b, indent, op, "}\n")
	} else {
		shouldPrintOld := shouldPrintPropertyValue(diff.Old, false)
		shouldPrintNew := shouldPrintPropertyValue(diff.New, false)

		if diff.Old.IsArchive() &&
			diff.New.IsArchive() &&
			!causedReplace &&
			shouldPrintOld &&
			shouldPrintNew {
			printArchiveDiff(b, titleFunc, diff.Old.ArchiveValue(), diff.New.ArchiveValue(), planning, indent)
			return
		}

		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintOld {
			printDelete(b, diff.Old, titleFunc, causedReplace, planning, indent)
		}
		if shouldPrintNew {
			printAdd(b, diff.New, titleFunc, causedReplace, planning, indent)
		}
	}
}

func printDelete(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	causedReplace bool, planning bool, indent int) {
	op := deploy.OpDelete
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true)
}

func printAdd(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	causedReplace bool, planning bool, indent int) {
	op := deploy.OpCreate
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true)
}

func printArchiveDiff(
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	oldArchive *resource.Archive, newArchive *resource.Archive,
	planning bool, indent int) {

	// TODO: this could be called recursively from itself.  In the recursive case, we might have an
	// archive that actually hasn't changed.  Check for that, and terminate the diff printing.

	op := deploy.OpUpdate

	hashChange := getTextChangeString(shortHash(oldArchive.Hash), shortHash(newArchive.Hash))

	if oldPath, has := oldArchive.GetPath(); has {
		if newPath, has := newArchive.GetPath(); has {
			titleFunc(op, true)
			write(b, op, "archive(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else if oldURI, has := oldArchive.GetURI(); has {
		if newURI, has := newArchive.GetURI(); has {
			titleFunc(op, true)
			write(b, op, "archive(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	} else {
		contract.Assert(oldArchive.IsAssets())
		oldAssets, _ := oldArchive.GetAssets()

		if newAssets, has := newArchive.GetAssets(); has {
			titleFunc(op, true)
			write(b, op, "archive(assets:%s) {\n", hashChange)
			printAssetsDiff(b, oldAssets, newAssets, planning, indent+1)
			writeWithIndentPrefix(b, indent, deploy.OpUpdate, "}\n")
			return
		}
	}

	// Type of archive changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldArchive),
		titleFunc, false /*causedReplace*/, planning, indent)
	printAdd(
		b, assetOrArchiveToPropertyValue(newArchive),
		titleFunc, false /*causedReplace*/, planning, indent)
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
				titleFunc := func(top deploy.StepOp, tprefix bool) {
					printPropertyTitle(b, "\""+oldName+"\"", maxkey, indent, top, tprefix)
				}

				oldAsset := oldAssets[oldName]
				newAsset := newAssets[newName]

				switch t := oldAsset.(type) {
				case *resource.Archive:
					printArchiveDiff(b, titleFunc, t, newAsset.(*resource.Archive), planning, indent)
				case *resource.Asset:
					printAssetDiff(b, titleFunc, t, newAsset.(*resource.Asset), planning, indent)
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
			titleFunc := func(top deploy.StepOp, tprefix bool) {
				printPropertyTitle(b, "\""+oldName+"\"", maxkey, indent, top, tprefix)
			}
			printDelete(b, assetOrArchiveToPropertyValue(oldAssets[oldName]), titleFunc, false, planning, newIndent)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			titleFunc := func(top deploy.StepOp, tprefix bool) {
				printPropertyTitle(b, "\""+newName+"\"", maxkey, indent, top, tprefix)
			}
			printAdd(b, assetOrArchiveToPropertyValue(newAssets[newName]), titleFunc, false, planning, newIndent)
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
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	oldAsset *resource.Asset, newAsset *resource.Asset,
	planning bool, indent int) {

	op := deploy.OpUpdate

	// If the assets aren't changed, just print out: = assetName: type(hash)
	if oldAsset.Hash == newAsset.Hash {
		op = deploy.OpSame
		titleFunc(op, false)
		write(b, op, makeAssetHeader(oldAsset))
		return
	}

	// if the asset changed, print out: ~ assetName: type(hash->hash) details...

	hashChange := getTextChangeString(shortHash(oldAsset.Hash), shortHash(newAsset.Hash))

	if oldText, has := oldAsset.GetText(); has {
		if newText, has := newAsset.GetText(); has {
			titleFunc(deploy.OpUpdate, true)
			write(b, op, "asset(text:%s) {\n", hashChange)

			massagedOldText := massageText(oldText)
			massagedNewText := massageText(newText)

			differ := diffmatchpatch.New()
			differ.DiffTimeout = 0

			hashed1, hashed2, lineArray := differ.DiffLinesToChars(massagedOldText, massagedNewText)
			diffs1 := differ.DiffMain(hashed1, hashed2, false)
			diffs2 := differ.DiffCharsToLines(diffs1, lineArray)

			b.WriteString(diffToPrettyString(diffs2, indent+1))

			writeWithIndentPrefix(b, indent, op, "}\n")
			return
		}
	} else if oldPath, has := oldAsset.GetPath(); has {
		if newPath, has := newAsset.GetPath(); has {
			titleFunc(deploy.OpUpdate, true)
			write(b, op, "asset(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else {
		contract.Assert(oldAsset.IsURI())

		oldURI, _ := oldAsset.GetURI()
		if newURI, has := newAsset.GetURI(); has {
			titleFunc(deploy.OpUpdate, true)
			write(b, op, "asset(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	}

	// Type of asset changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldAsset),
		titleFunc, false /*causedReplace*/, planning, indent)
	printAdd(
		b, assetOrArchiveToPropertyValue(newAsset),
		titleFunc, false /*causedReplace*/, planning, indent)
}

func getTextChangeString(old string, new string) string {
	if old == new {
		return old
	}

	return fmt.Sprintf("%s->%s", old, new)
}

var (
	shaRegexp    = regexp.MustCompile("__[a-zA-Z0-9]{40}")
	pragmaRegexp = regexp.MustCompile(`(?s)/\* <auto-generated>(.*?)</auto-generated> \*/`)
)

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

	// Only do this for strings that match our serialized function pattern.
	if !pragmaRegexp.MatchString(text) {
		return text
	}

	replaceNewlines := func() {
		for {
			newText := strings.Replace(text, "\n\n\n", "\n\n", -1)
			if len(newText) == len(text) {
				break
			}

			text = newText
		}
	}

	replaceNewlines()

	text = shaRegexp.ReplaceAllString(text, "__shaHash")
	text = pragmaRegexp.ReplaceAllString(text, "")

	replaceNewlines()

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
		var prefix bool
		if op == deploy.OpCreate || op == deploy.OpDelete {
			prefix = true
		}
		writeWithIndent(&buff, indent, op, prefix, "%s", text)
	}

	for index, diff := range diffs {
		text := diff.Text
		lines := strings.Split(text, "\n")
		printLines := func(op deploy.StepOp, startInclusive int, endExclusive int) {
			for i := startInclusive; i < endExclusive; i++ {
				if strings.TrimSpace(lines[i]) != "" {
					writeDiff(op, lines[i])
					buff.WriteString("\n")
				}
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

			const contextLines = 2

			// Show the unchanged text in white.
			if index == 0 {
				// First chunk of the file.
				if len(lines) > contextLines+1 {
					writeDiff(deploy.OpSame, "...\n")
					printLines(deploy.OpSame, len(lines)-contextLines, len(lines))
					continue
				}
			} else if index == len(diffs)-1 {
				if len(lines) > contextLines+1 {
					printLines(deploy.OpSame, 0, contextLines)
					writeDiff(deploy.OpSame, "...\n")
					continue
				}
			} else {
				if len(lines) > (2*contextLines + 1) {
					printLines(deploy.OpSame, 0, contextLines)
					writeDiff(deploy.OpSame, "...\n")
					printLines(deploy.OpSame, len(lines)-contextLines, len(lines))
					continue
				}
			}

			printLines(deploy.OpSame, 0, len(lines))
		}
	}

	return buff.String()
}

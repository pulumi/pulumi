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

	"github.com/opentracing/opentracing-go"
	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, config plugin.ConfigSource, diag diag.Sink,
	tracingSpan opentracing.Span) (string, string, *plugin.Context, error) {
	contract.Require(projinfo != nil, "projinfo")

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return "", "", nil, err
	}

	// Create a context for plugins.
	ctx, err := plugin.NewContext(diag, nil, config, pwd, tracingSpan)
	if err != nil {
		return "", "", nil, err
	}

	return pwd, main, ctx, nil
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(info *planContext, opts deployOptions) (*planResult, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Update != nil)

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := info.Update.GetProject(), info.Update.GetTarget()
	contract.Assert(proj != nil)
	contract.Assert(target != nil)
	projinfo := &Projinfo{Proj: proj, Root: info.Update.GetRoot()}
	pwd, main, ctx, err := ProjectInfoContext(projinfo, target, opts.Diag, info.TracingSpan)
	if err != nil {
		return nil, err
	}

	// Figure out which plugins to load.  In the case of a destroy, we consult the manifest for the plugin versions
	// required to destroy it.  Otherwise, we inspect the program contents to figure out which will be required.
	var plugins []workspace.PluginInfo
	if opts.Destroy {
		if target.Snapshot != nil {
			plugins = target.Snapshot.Manifest.Plugins
		}
	} else {
		if plugins, err = ctx.Host.GetRequiredPlugins(plugin.ProgInfo{
			Proj:    proj,
			Pwd:     pwd,
			Program: main,
		}); err != nil {
			return nil, err
		}
	}

	// Now ensure that we have loaded up any plugins that the program will need in advance.
	err = ctx.Host.EnsurePlugins(plugins)
	if err != nil {
		return nil, err
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	source := deploy.NewEvalSource(ctx, &deploy.EvalRunInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
		Target:  target,
	}, opts.Destroy, opts.DryRun)

	// If there are any analyzers in the project file, add them.
	var analyzers []tokens.QName
	if as := projinfo.Proj.Analyzers; as != nil {
		for _, a := range *as {
			analyzers = append(analyzers, a)
		}
	}

	// Append any analyzers from the command line.
	for _, a := range opts.Analyzers {
		analyzers = append(analyzers, tokens.QName(a))
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan := deploy.NewPlan(ctx, target, target.Snapshot, source, analyzers, opts.DryRun)
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

// printPlan prints the plan's result to the plan's Options.Events stream.
func printPlan(result *planResult) (ResourceChanges, error) {
	result.Options.Events.preludeEvent(result.Options.DryRun,
		result.Info.Update.GetTarget().Config)

	// Walk the plan's steps and and pretty-print them out.
	actions := newPreviewActions(result.Options)
	_, _, _, err := result.Walk(actions, true)
	if err != nil {
		return nil, errors.New("an error occurred while advancing the preview")
	}

	if !result.Options.Diag.Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return nil, errors.New("one or more errors occurred during this preview")
	}

	// Emit an event with a summary of operation counts.
	changes := ResourceChanges(actions.Ops)
	result.Options.Events.previewSummaryEvent(changes)
	return changes, nil
}

func assertSeen(seen map[resource.URN]deploy.Step, step deploy.Step) {
	_, has := seen[step.URN()]
	contract.Assertf(has, "URN '%v' had not been marked as seen", step.URN())
}

// getIndent computes a step's parent indentation.
func getIndent(step deploy.Step, seen map[resource.URN]deploy.Step) int {
	indent := 0
	for p := step.Res().Parent; p != ""; {
		par := seen[p]
		if par == nil {
			// This can happen during deletes, since we delete children before parents.
			// TODO[pulumi/pulumi#340]: we need to figure out how best to display this sequence; at the very
			//     least, it would be ideal to preserve the indentation.
			break
		}
		indent++
		p = par.Res().Parent
	}
	return indent
}

func printStepHeader(b *bytes.Buffer, step deploy.Step) {
	var extra string
	old := step.Old()
	new := step.New()
	if new != nil && !new.Protect && old != nil && old.Protect {
		// show an unlocked symbol, since we are unprotecting a resource.
		extra = " 🔓"
	} else if (new != nil && new.Protect) || (old != nil && old.Protect) {
		// show a locked symbol, since we are either newly protecting this resource, or retaining protection.
		extra = " 🔒"
	}
	b.WriteString(fmt.Sprintf("%s: (%s)%s\n", string(step.Type()), step.Op(), extra))
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

func writeWithIndentNoPrefix(b *bytes.Buffer, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, indent, op, false, format, a...)
}

func write(b *bytes.Buffer, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndentNoPrefix(b, 0, op, format, a...)
}

func writeVerbatim(b *bytes.Buffer, op deploy.StepOp, value string) {
	writeWithIndentNoPrefix(b, 0, op, "%s", value)
}

func getResourcePropertiesSummary(step deploy.Step, indent int) string {
	var b bytes.Buffer

	op := step.Op()
	urn := step.URN()
	old := step.Old()

	// Print the indentation.
	b.WriteString(getIndentationString(indent, op, false))

	// First, print out the operation's prefix.
	b.WriteString(op.Prefix())

	// Next, print the resource type (since it is easy on the eyes and can be quickly identified).
	printStepHeader(&b, step)

	// For these simple properties, print them as 'same' if they're just an update or replace.
	simplePropOp := considerSameIfNotCreateOrDelete(op)

	// Print out the URN and, if present, the ID, as "pseudo-properties" and indent them.
	var id resource.ID
	if old != nil {
		id = old.ID
	}

	// Always print the ID and URN.
	if id != "" {
		writeWithIndentNoPrefix(&b, indent+1, simplePropOp, "[id=%s]\n", string(id))
	}
	if urn != "" {
		writeWithIndentNoPrefix(&b, indent+1, simplePropOp, "[urn=%s]\n", urn)
	}

	return b.String()
}

func getResourcePropertiesDetails(step deploy.Step, indent int, planning bool, debug bool) string {
	var b bytes.Buffer

	// indent everything an additional level, like other properties.
	indent++

	var replaces []resource.PropertyKey
	if step.Op() == deploy.OpCreateReplacement {
		replaces = step.(*deploy.CreateStep).Keys()
	} else if step.Op() == deploy.OpReplace {
		replaces = step.(*deploy.ReplaceStep).Keys()
	}

	old := step.Old()
	new := step.New()

	if old == nil && new != nil {
		printObject(&b, new.Inputs, planning, indent, step.Op(), false, debug)
	} else if new == nil && old != nil {
		printObject(&b, old.Inputs, planning, indent, step.Op(), false, debug)
	} else {
		printOldNewDiffs(&b, old.Inputs, new.Inputs, replaces, planning, indent, step, debug)
	}

	return b.String()
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
	indent int, op deploy.StepOp, prefix bool, debug bool) {

	// Compute the maximum with of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v, planning) {
			printPropertyTitle(b, string(k), maxkey, indent, op, prefix)
			printPropertyValue(b, v, planning, indent, op, prefix, debug)
		}
	}
}

// printResourceOutputProperties prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func getResourceOutputsPropertiesString(
	step deploy.Step, indent int, planning bool, debug bool) string {
	var b bytes.Buffer

	// Only certain kinds of steps have output properties associated with them.
	new := step.New()
	if new == nil || new.Outputs == nil {
		return ""
	}
	op := considerSameIfNotCreateOrDelete(step.Op())

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
				print = (out.Diff(in, planning, step.Stables()) != nil)
			} else {
				print = true
			}

			if print {
				if firstout {
					writeWithIndentNoPrefix(&b, indent, op, "---outputs:---\n")
					firstout = false
				}
				printPropertyTitle(&b, string(k), maxkey, indent, op, false)
				printPropertyValue(&b, out, planning, indent, op, false, debug)
			}
		}
	}

	return b.String()
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
	indent int, op deploy.StepOp, prefix bool, debug bool) {

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
				printPropertyValue(b, elem, planning, indent+1, op, prefix, debug)
			}
			writeWithIndentNoPrefix(b, indent, op, "]")
		}
	} else if v.IsAsset() {
		a := v.AssetValue()
		if text, has := a.GetText(); has {
			write(b, op, "asset(text:%s) {\n", shortHash(a.Hash))

			massaged := massageText(text, debug)

			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(massaged, "\n")
			for _, line := range lines {
				writeWithIndentNoPrefix(b, indent, op, "    %s\n", line)
			}
			writeWithIndentNoPrefix(b, indent, op, "}")
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
				printAssetOrArchive(b, assets[name], name, planning, indent, op, prefix, debug)
			}
			writeWithIndentNoPrefix(b, indent, op, "}")
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
			printObject(b, obj, planning, indent+1, op, prefix, debug)
			writeWithIndentNoPrefix(b, indent, op, "}")
		}
	}
	writeVerbatim(b, op, "\n")
}

func printAssetOrArchive(
	b *bytes.Buffer, v interface{}, name string, planning bool,
	indent int, op deploy.StepOp, prefix bool, debug bool) {
	writeWithIndent(b, indent, op, prefix, "    \"%v\": ", name)
	printPropertyValue(b, assetOrArchiveToPropertyValue(v), planning, indent+1, op, prefix, debug)
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
	replaces []resource.PropertyKey, planning bool, indent int, step deploy.Step, debug bool) {

	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news, planning, step.Stables()); diff != nil {
		printObjectDiff(b, *diff, replaces, false, planning, indent, debug)
	} else {
		printObject(b, news, planning, indent, step.Op(), true, debug)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff,
	replaces []resource.PropertyKey, causedReplace bool, planning bool, indent int, debug bool) {

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
				printAdd(b, add, titleFunc, true, planning, indent, debug)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete, planning) {
				printDelete(b, delete, titleFunc, true, planning, indent, debug)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}

			printPropertyValueDiff(b, titleFunc, update, causedReplace, planning, indent, debug)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same, planning) {
			titleFunc(deploy.OpSame, false)
			printPropertyValue(b, diff.Sames[k], planning, indent, deploy.OpSame, false, debug)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	diff resource.ValueDiff, causedReplace bool, planning bool, indent int, debug bool) {

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
				printAdd(b, add, elemTitleFunc, true, planning, indent+2, debug)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				printDelete(b, delete, elemTitleFunc, true, planning, indent+2, debug)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(b, elemTitleFunc, update, causedReplace, planning, indent+2, debug)
			} else {
				elemTitleFunc(deploy.OpSame, false)
				printPropertyValue(b, a.Sames[i], planning, indent+2, deploy.OpSame, false, debug)
			}
		}
		writeWithIndentNoPrefix(b, indent, op, "]\n")
	} else if diff.Object != nil {
		titleFunc(op, true)
		writeVerbatim(b, op, "{\n")
		printObjectDiff(b, *diff.Object, nil, causedReplace, planning, indent+1, debug)
		writeWithIndentNoPrefix(b, indent, op, "}\n")
	} else {
		shouldPrintOld := shouldPrintPropertyValue(diff.Old, false)
		shouldPrintNew := shouldPrintPropertyValue(diff.New, false)

		if diff.Old.IsArchive() &&
			diff.New.IsArchive() &&
			!causedReplace &&
			shouldPrintOld &&
			shouldPrintNew {
			printArchiveDiff(b, titleFunc, diff.Old.ArchiveValue(), diff.New.ArchiveValue(), planning, indent, debug)
			return
		}

		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintOld {
			printDelete(b, diff.Old, titleFunc, causedReplace, planning, indent, debug)
		}
		if shouldPrintNew {
			printAdd(b, diff.New, titleFunc, causedReplace, planning, indent, debug)
		}
	}
}

func printDelete(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	causedReplace bool, planning bool, indent int, debug bool) {
	op := deploy.OpDelete
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true, debug)
}

func printAdd(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	causedReplace bool, planning bool, indent int, debug bool) {
	op := deploy.OpCreate
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true, debug)
}

func printArchiveDiff(
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	oldArchive *resource.Archive, newArchive *resource.Archive,
	planning bool, indent int, debug bool) {

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
			printAssetsDiff(b, oldAssets, newAssets, planning, indent+1, debug)
			writeWithIndentNoPrefix(b, indent, deploy.OpUpdate, "}\n")
			return
		}
	}

	// Type of archive changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldArchive),
		titleFunc, false /*causedReplace*/, planning, indent, debug)
	printAdd(
		b, assetOrArchiveToPropertyValue(newArchive),
		titleFunc, false /*causedReplace*/, planning, indent, debug)
}

func printAssetsDiff(
	b *bytes.Buffer,
	oldAssets map[string]interface{}, newAssets map[string]interface{},
	planning bool, indent int, debug bool) {

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
					printArchiveDiff(b, titleFunc, t, newAsset.(*resource.Archive), planning, indent, debug)
				case *resource.Asset:
					printAssetDiff(b, titleFunc, t, newAsset.(*resource.Asset), planning, indent, debug)
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
			printDelete(b, assetOrArchiveToPropertyValue(oldAssets[oldName]), titleFunc, false, planning, newIndent, debug)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			titleFunc := func(top deploy.StepOp, tprefix bool) {
				printPropertyTitle(b, "\""+newName+"\"", maxkey, indent, top, tprefix)
			}
			printAdd(b, assetOrArchiveToPropertyValue(newAssets[newName]), titleFunc, false, planning, newIndent, debug)
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
	planning bool, indent int, debug bool) {

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

			massagedOldText := massageText(oldText, debug)
			massagedNewText := massageText(newText, debug)

			differ := diffmatchpatch.New()
			differ.DiffTimeout = 0

			hashed1, hashed2, lineArray := differ.DiffLinesToChars(massagedOldText, massagedNewText)
			diffs1 := differ.DiffMain(hashed1, hashed2, false)
			diffs2 := differ.DiffCharsToLines(diffs1, lineArray)

			b.WriteString(diffToPrettyString(diffs2, indent+1))

			writeWithIndentNoPrefix(b, indent, op, "}\n")
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
		titleFunc, false /*causedReplace*/, planning, indent, debug)
	printAdd(
		b, assetOrArchiveToPropertyValue(newAsset),
		titleFunc, false /*causedReplace*/, planning, indent, debug)
}

func getTextChangeString(old string, new string) string {
	if old == new {
		return old
	}

	return fmt.Sprintf("%s->%s", old, new)
}

var (
	functionRegexp    = regexp.MustCompile(`function __.*`)
	withRegexp        = regexp.MustCompile(`    with\({ .* }\) {`)
	environmentRegexp = regexp.MustCompile(`  }\).apply\(.*\).apply\(this, arguments\);`)
	preambleRegexp    = regexp.MustCompile(
		`function __.*\(\) {\n  return \(function\(\) {\n    with \(__closure\) {\n\nreturn `)
	postambleRegexp = regexp.MustCompile(
		`;\n\n    }\n  }\).apply\(__environment\).apply\(this, arguments\);\n}`)
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
func massageText(text string, debug bool) string {
	if debug {
		return text
	}

	// Only do this for strings that match our serialized function pattern.
	if !functionRegexp.MatchString(text) ||
		!withRegexp.MatchString(text) ||
		!environmentRegexp.MatchString(text) {

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

	firstFunc := functionRegexp.FindStringIndex(text)
	text = text[firstFunc[0]:]

	text = withRegexp.ReplaceAllString(text, "    with (__closure) {")
	text = environmentRegexp.ReplaceAllString(text, "  }).apply(__environment).apply(this, arguments);")

	text = preambleRegexp.ReplaceAllString(text, "")
	text = postambleRegexp.ReplaceAllString(text, "")

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

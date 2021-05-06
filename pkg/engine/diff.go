// Copyright 2016-2018, Pulumi Corporation.
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

package engine

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// GetIndent computes a step's parent indentation.
func GetIndent(step StepEventMetadata, seen map[resource.URN]StepEventMetadata) int {
	indent := 0
	for p := step.Res.Parent; p != ""; {
		if par, has := seen[p]; !has {
			// This can happen during deletes, since we delete children before parents.
			// TODO[pulumi/pulumi#340]: we need to figure out how best to display this sequence; at the very
			//     least, it would be ideal to preserve the indentation.
			break
		} else {
			indent++
			p = par.Res.Parent
		}
	}
	return indent
}

func printStepHeader(b io.StringWriter, step StepEventMetadata) {
	var extra string
	old := step.Old
	new := step.New
	if new != nil && !new.Protect && old != nil && old.Protect {
		// show an unlocked symbol, since we are unprotecting a resource.
		extra = " 🔓"
	} else if (new != nil && new.Protect) || (old != nil && old.Protect) {
		// show a locked symbol, since we are either newly protecting this resource, or retaining protection.
		extra = " 🔒"
	}
	writeString(b, fmt.Sprintf("%s: (%s)%s\n", string(step.Type), step.Op, extra))
}

func GetIndentationString(indent int) string {
	var result string
	for i := 0; i < indent; i++ {
		result += "    "
	}
	return result
}

func getIndentationString(indent int, op deploy.StepOp, prefix bool) string {
	var result = GetIndentationString(indent)

	if !prefix {
		return result
	}

	if result == "" {
		contract.Assertf(!prefix, "Expected indention for a prefixed line")
		return result
	}

	rp := op.RawPrefix()
	contract.Assert(len(rp) == 2)
	contract.Assert(len(result) >= 2)
	return result[:len(result)-2] + rp
}

func writeString(b io.StringWriter, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}

func writeWithIndent(b io.StringWriter, indent int, op deploy.StepOp, prefix bool, format string, a ...interface{}) {
	writeString(b, op.Color())
	writeString(b, getIndentationString(indent, op, prefix))
	writeString(b, fmt.Sprintf(format, a...))
	writeString(b, colors.Reset)
}

func writeWithIndentNoPrefix(b io.StringWriter, indent int, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, indent, op, false, format, a...)
}

func write(b io.StringWriter, op deploy.StepOp, format string, a ...interface{}) {
	writeWithIndentNoPrefix(b, 0, op, format, a...)
}

func writeVerbatim(b io.StringWriter, op deploy.StepOp, value string) {
	writeWithIndentNoPrefix(b, 0, op, "%s", value)
}

func GetResourcePropertiesSummary(step StepEventMetadata, indent int) string {
	var b bytes.Buffer

	op := step.Op
	urn := step.URN
	old := step.Old

	// Print the indentation.
	writeString(&b, getIndentationString(indent, op, false))

	// First, print out the operation's prefix.
	writeString(&b, op.Prefix())

	// Next, print the resource type (since it is easy on the eyes and can be quickly identified).
	printStepHeader(&b, step)

	// For these simple properties, print them as 'same' if they're just an update or replace.
	simplePropOp := considerSameIfNotCreateOrDelete(op)

	// Print out the URN and, if present, the ID, as "pseudo-properties" and indent them.
	var id resource.ID
	if old != nil {
		id = old.ID
	}

	// Always print the ID, URN, and provider.
	if id != "" {
		writeWithIndentNoPrefix(&b, indent+1, simplePropOp, "[id=%s]\n", string(id))
	}
	if urn != "" {
		writeWithIndentNoPrefix(&b, indent+1, simplePropOp, "[urn=%s]\n", urn)
	}

	if step.Provider != "" {
		new := step.New
		if old != nil && new != nil && old.Provider != new.Provider {
			newProv, err := providers.ParseReference(new.Provider)
			contract.Assert(err == nil)

			writeWithIndentNoPrefix(&b, indent+1, deploy.OpUpdate, "[provider: ")
			write(&b, deploy.OpDelete, "%s", old.Provider)
			writeVerbatim(&b, deploy.OpUpdate, " => ")
			if newProv.ID() == providers.UnknownID {
				write(&b, deploy.OpCreate, "%s", string(newProv.URN())+"::output<string>")
			} else {
				write(&b, deploy.OpCreate, "%s", new.Provider)
			}
			writeVerbatim(&b, deploy.OpUpdate, "]\n")
		} else {
			prov, err := providers.ParseReference(step.Provider)
			contract.Assert(err == nil)

			// Elide references to default providers.
			if prov.URN().Name() != "default" {
				writeWithIndentNoPrefix(&b, indent+1, simplePropOp, "[provider=%s]\n", step.Provider)
			}
		}
	}

	return b.String()
}

func GetResourcePropertiesDetails(
	step StepEventMetadata, indent int, planning bool, summary bool, debug bool) string {
	var b bytes.Buffer

	// indent everything an additional level, like other properties.
	indent++

	old, new := step.Old, step.New
	if old == nil && new != nil {
		if len(new.Outputs) > 0 {
			PrintObject(&b, new.Outputs, planning, indent, step.Op, false, debug)
		} else {
			PrintObject(&b, new.Inputs, planning, indent, step.Op, false, debug)
		}
	} else if new == nil && old != nil {
		// in summary view, we don't have to print out the entire object that is getting deleted.
		// note, the caller will have already printed out the type/name/id/urn of the resource,
		// and that's sufficient for a summarized deletion view.
		if !summary {
			PrintObject(&b, old.Inputs, planning, indent, step.Op, false, debug)
		}
	} else if len(new.Outputs) > 0 && step.Op != deploy.OpImport && step.Op != deploy.OpImportReplacement {
		printOldNewDiffs(&b, old.Outputs, new.Outputs, nil, planning, indent, step.Op, summary, debug)
	} else {
		printOldNewDiffs(&b, old.Inputs, new.Inputs, step.Diffs, planning, indent, step.Op, summary, debug)
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

func PrintObject(
	b *bytes.Buffer, props resource.PropertyMap, planning bool,
	indent int, op deploy.StepOp, prefix bool, debug bool) {

	// Compute the maximum width of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; !resource.IsInternalPropertyKey(k) && shouldPrintPropertyValue(v, planning) {
			printPropertyTitle(b, string(k), maxkey, indent, op, prefix)
			printPropertyValue(b, v, planning, indent, op, prefix, debug)
		}
	}
}

func PrintResourceReference(
	b *bytes.Buffer, resRef resource.ResourceReference, planning bool,
	indent int, op deploy.StepOp, prefix bool, debug bool) {

	printPropertyTitle(b, "URN", 3, indent, op, prefix)
	write(b, op, "%q\n", resRef.URN)
	printPropertyTitle(b, "ID", 3, indent, op, prefix)
	printPropertyValue(b, resRef.ID, planning, indent, op, prefix, debug)
	printPropertyTitle(b, "PackageVersion", 3, indent, op, prefix)
	write(b, op, "%q\n", resRef.PackageVersion)
}

func massageStackPreviewAdd(p resource.PropertyValue) resource.PropertyValue {
	switch {
	case p.IsArray():
		arr := make([]resource.PropertyValue, len(p.ArrayValue()))
		for i, v := range p.ArrayValue() {
			arr[i] = massageStackPreviewAdd(v)
		}
		return resource.NewArrayProperty(arr)
	case p.IsObject():
		obj := resource.PropertyMap{}
		for k, v := range p.ObjectValue() {
			if k != "@isPulumiResource" {
				obj[k] = massageStackPreviewAdd(v)
			}
		}
		return resource.NewObjectProperty(obj)
	default:
		return p
	}
}

func massageStackPreviewDiff(diff resource.ValueDiff, inResource bool) {
	switch {
	case diff.Array != nil:
		for i, p := range diff.Array.Adds {
			diff.Array.Adds[i] = massageStackPreviewAdd(p)
		}
		for _, d := range diff.Array.Updates {
			massageStackPreviewDiff(d, inResource)
		}
	case diff.Object != nil:
		massageStackPreviewOutputDiff(diff.Object, inResource)
	}
}

// massageStackPreviewOutputDiff removes any adds of unknown values nested inside Pulumi resources present in a stack's
// outputs.
func massageStackPreviewOutputDiff(diff *resource.ObjectDiff, inResource bool) {
	if diff == nil {
		return
	}

	_, isResource := diff.Adds["@isPulumiResource"]
	if isResource {
		delete(diff.Adds, "@isPulumiResource")

		for k, v := range diff.Adds {
			if v.IsComputed() {
				delete(diff.Adds, k)
			}
		}
	}

	for i, p := range diff.Adds {
		diff.Adds[i] = massageStackPreviewAdd(p)
	}
	for k, d := range diff.Updates {
		if isResource && d.New.IsComputed() && !shouldPrintPropertyValue(d.Old, false) {
			delete(diff.Updates, k)
		} else {
			massageStackPreviewDiff(d, inResource)
		}
	}
}

// GetResourceOutputsPropertiesString prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func GetResourceOutputsPropertiesString(
	step StepEventMetadata, indent int, planning, debug, refresh, showSames bool) string {

	// During the actual update we always show all the outputs for the stack, even if they are unchanged.
	if !showSames && !planning && step.URN.Type() == resource.RootStackType {
		showSames = true
	}

	// We should only print outputs for normal resources if the outputs are known to be complete.
	// This will be the case if we are:
	//
	//   1) not doing a preview
	//   2) doing a refresh
	//   3) doing a read
	//   4) doing an import
	//
	// Technically, 2-4 are the same, since they're all bottoming out at a provider's implementation
	// of Read, but the upshot is that either way we're ending up with outputs that are exactly
	// accurate. If we are not sure that we are in one of the above states, we shouldn't try to
	// print outputs.
	//
	// Note: we always show the outputs for the stack itself.  These are valuable enough to want
	// to always see.
	if planning {
		printOutputDuringPlanning := refresh ||
			step.Op == deploy.OpRead ||
			step.Op == deploy.OpReadReplacement ||
			step.Op == deploy.OpImport ||
			step.Op == deploy.OpImportReplacement ||
			step.URN.Type() == resource.RootStackType
		if !printOutputDuringPlanning {
			return ""
		}
	}

	// Resources that have initialization errors did not successfully complete, and therefore do not
	// have outputs to render diffs for. So, simply return.
	if step.Old != nil && len(step.Old.InitErrors) > 0 {
		return ""
	}

	// Only certain kinds of steps have output properties associated with them.
	var ins resource.PropertyMap
	var outs resource.PropertyMap
	if step.New == nil || step.New.Outputs == nil {
		ins = make(resource.PropertyMap)
		outs = make(resource.PropertyMap)
	} else {
		ins = step.New.Inputs
		outs = step.New.Outputs
	}
	op := step.Op

	// If there was an old state associated with this step, we may have old outputs. If we do, and if they differ from
	// the new outputs, we want to print the diffs.
	var outputDiff *resource.ObjectDiff
	if step.Old != nil && step.Old.Outputs != nil {
		outputDiff = step.Old.Outputs.Diff(outs, resource.IsInternalPropertyKey)

		// If this is the root stack type, we want to strip out any nested resource outputs that are not known if
		// they have no corresponding output in the old state.
		if planning && step.URN.Type() == resource.RootStackType {
			massageStackPreviewOutputDiff(outputDiff, false)
		}
	}

	// If we asked to not show-sames, and no outputs changed then don't show anything at all here.
	if outputDiff == nil && !showSames {
		return ""
	}

	var keys []resource.PropertyKey
	if outputDiff == nil {
		keys = outs.StableKeys()
	} else {
		keys = outputDiff.Keys()
	}
	maxkey := maxKey(keys)

	b := &bytes.Buffer{}

	// Now sort the keys and enumerate each output property in a deterministic order.
	for _, k := range keys {
		out := outs[k]

		// Print this property if it is printable and if any of the following are true:
		// - a property with the same key is not present in the inputs
		// - the property that is present in the inputs is different
		// - we are doing a refresh, in which case we always want to show state differences
		if outputDiff != nil || (!resource.IsInternalPropertyKey(k) && shouldPrintPropertyValue(out, true)) {
			if in, has := ins[k]; has && !refresh {
				if out.Diff(in, resource.IsInternalPropertyKey) == nil {
					continue
				}
			}

			// If we asked to not show-sames, and this is a same output, then filter it out of what
			// we display.
			if !showSames && outputDiff != nil && outputDiff.Same(k) {
				continue
			}

			if outputDiff != nil {
				printObjectPropertyDiff(b, k, maxkey, *outputDiff, planning, indent, false, debug)
			} else {
				printPropertyTitle(b, string(k), maxkey, indent, op, false)
				printPropertyValue(b, out, planning, indent, op, false, debug)
			}
		}
	}

	return b.String()
}

func considerSameIfNotCreateOrDelete(op deploy.StepOp) deploy.StepOp {
	switch op {
	case deploy.OpCreate, deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
		return op
	default:
		return deploy.OpSame
	}
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

func printPropertyTitle(b io.StringWriter, name string, align int, indent int, op deploy.StepOp, prefix bool) {
	writeWithIndent(b, indent, op, prefix, "%-"+strconv.Itoa(align)+"s: ", name)
}

func printPropertyValue(
	b *bytes.Buffer, v resource.PropertyValue, planning bool,
	indent int, op deploy.StepOp, prefix bool, debug bool) {

	switch {
	case isPrimitive(v) || v.IsSecret():
		printPrimitivePropertyValue(b, v, planning, op)
	case v.IsArray():
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
	case v.IsAsset():
		a := v.AssetValue()
		if a.IsText() {
			write(b, op, "asset(text:%s) {\n", shortHash(a.Hash))

			a = resource.MassageIfUserProgramCodeAsset(a, debug)

			massaged := a.Text

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
	case v.IsArchive():
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
	case v.IsObject():
		obj := v.ObjectValue()
		if len(obj) == 0 {
			writeVerbatim(b, op, "{}")
		} else {
			writeVerbatim(b, op, "{\n")
			PrintObject(b, obj, planning, indent+1, op, prefix, debug)
			writeWithIndentNoPrefix(b, indent, op, "}")
		}
	case v.IsResourceReference():
		resRef := v.ResourceReferenceValue()
		writeVerbatim(b, op, "{\n")
		PrintResourceReference(b, resRef, planning, indent+1, op, prefix, debug)
		writeWithIndentNoPrefix(b, indent, op, "}")
	default:
		contract.Failf("Unknown PropertyValue type %v", v)
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
	b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap, include []resource.PropertyKey,
	planning bool, indent int, op deploy.StepOp, summary bool, debug bool) {

	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news, resource.IsInternalPropertyKey); diff != nil {
		PrintObjectDiff(b, *diff, include, planning, indent, summary, debug)
	} else {
		// If there's no diff, report the op as Same - there's no diff to render
		// so it should be rendered as if nothing changed.
		PrintObject(b, news, planning, indent, deploy.OpSame, true, debug)
	}
}

func PrintObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff, include []resource.PropertyKey,
	planning bool, indent int, summary bool, debug bool) {

	contract.Assert(indent > 0)

	// Compute the maximum width of property keys so we can justify everything. If an include set was given, filter out
	// any properties that are not in the set.
	keys := diff.Keys()
	if include != nil {
		includeSet := make(map[resource.PropertyKey]bool)
		for _, k := range include {
			includeSet[k] = true
		}
		var filteredKeys []resource.PropertyKey
		for _, k := range keys {
			if includeSet[k] {
				filteredKeys = append(filteredKeys, k)
			}
		}
		keys = filteredKeys
	}
	maxkey := maxKey(keys)

	// To print an object diff, enumerate the keys in stable order, and print each property independently.
	for _, k := range keys {
		printObjectPropertyDiff(b, k, maxkey, diff, planning, indent, summary, debug)
	}
}

func printObjectPropertyDiff(b *bytes.Buffer, key resource.PropertyKey, maxkey int, diff resource.ObjectDiff,
	planning bool, indent int, summary bool, debug bool) {

	titleFunc := func(top deploy.StepOp, prefix bool) {
		printPropertyTitle(b, string(key), maxkey, indent, top, prefix)
	}
	if add, isadd := diff.Adds[key]; isadd {
		printAdd(b, add, titleFunc, planning, indent, debug)
	} else if delete, isdelete := diff.Deletes[key]; isdelete {
		printDelete(b, delete, titleFunc, planning, indent, debug)
	} else if update, isupdate := diff.Updates[key]; isupdate {
		printPropertyValueDiff(
			b, titleFunc, update, planning, indent, summary, debug)
	} else if same := diff.Sames[key]; !summary && shouldPrintPropertyValue(same, planning) {
		titleFunc(deploy.OpSame, false)
		printPropertyValue(b, diff.Sames[key], planning, indent, deploy.OpSame, false, debug)
	}
}

func printPropertyValueDiff(
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	diff resource.ValueDiff, planning bool,
	indent int, summary bool, debug bool) {

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
				printAdd(b, add, elemTitleFunc, planning, indent+2, debug)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				printDelete(b, delete, elemTitleFunc, planning, indent+2, debug)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(
					b, elemTitleFunc, update, planning,
					indent+2, summary, debug)
			} else if !summary {
				elemTitleFunc(deploy.OpSame, false)
				printPropertyValue(b, a.Sames[i], planning, indent+2, deploy.OpSame, false, debug)
			}
		}
		writeWithIndentNoPrefix(b, indent, op, "]\n")
	} else if diff.Object != nil {
		titleFunc(op, true)
		writeVerbatim(b, op, "{\n")
		PrintObjectDiff(b, *diff.Object, nil, planning, indent+1, summary, debug)
		writeWithIndentNoPrefix(b, indent, op, "}\n")
	} else {
		shouldPrintOld := shouldPrintPropertyValue(diff.Old, false)
		shouldPrintNew := shouldPrintPropertyValue(diff.New, false)

		if shouldPrintOld && shouldPrintNew {
			if diff.Old.IsArchive() &&
				diff.New.IsArchive() {

				printArchiveDiff(
					b, titleFunc, diff.Old.ArchiveValue(), diff.New.ArchiveValue(),
					planning, indent, summary, debug)
				return
			}

			if isPrimitive(diff.Old) && isPrimitive(diff.New) {
				titleFunc(deploy.OpUpdate, true /*indent*/)
				printPrimitivePropertyValue(b, diff.Old, planning, deploy.OpDelete)
				writeVerbatim(b, deploy.OpUpdate, " => ")
				printPrimitivePropertyValue(b, diff.New, planning, deploy.OpCreate)
				writeVerbatim(b, deploy.OpUpdate, "\n")
				return
			}
		}

		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintOld {
			printDelete(b, diff.Old, titleFunc, planning, indent, debug)
		}
		if shouldPrintNew {
			printAdd(b, diff.New, titleFunc, planning, indent, debug)
		}
	}
}

func isPrimitive(value resource.PropertyValue) bool {
	return value.IsNull() || value.IsString() || value.IsNumber() ||
		value.IsBool() || value.IsComputed() || value.IsOutput()
}

func printPrimitivePropertyValue(b io.StringWriter, v resource.PropertyValue, planning bool, op deploy.StepOp) {
	contract.Assert(isPrimitive(v))

	if v.IsNull() {
		writeVerbatim(b, op, "<null>")
	} else if v.IsBool() {
		write(b, op, "%t", v.BoolValue())
	} else if v.IsNumber() {
		write(b, op, "%v", v.NumberValue())
	} else if v.IsString() {
		write(b, op, "%q", v.StringValue())
	} else if v.IsComputed() || v.IsOutput() {
		// We render computed and output values differently depending on whether or not we are
		// planning or deploying: in the former case, we display `computed<type>` or `output<type>`;
		// in the former we display `undefined`. This is because we currently cannot distinguish
		// between user-supplied undefined values and input properties that are undefined because
		// they were sourced from undefined values in other resources' output properties. Once we
		// have richer information about the dataflow between resources, we should be able to do a
		// better job here (pulumi/pulumi#234).
		if planning {
			writeVerbatim(b, op, v.TypeString())
		} else {
			write(b, op, "undefined")
		}
	} else {
		contract.Failf("Unexpected property value kind '%v'", v)
	}
}

func printDelete(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	planning bool, indent int, debug bool) {
	op := deploy.OpDelete
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true, debug)
}

func printAdd(
	b *bytes.Buffer, v resource.PropertyValue, title func(deploy.StepOp, bool),
	planning bool, indent int, debug bool) {
	op := deploy.OpCreate
	title(op, true)
	printPropertyValue(b, v, planning, indent, op, true, debug)
}

func printArchiveDiff(
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	oldArchive *resource.Archive, newArchive *resource.Archive,
	planning bool, indent int, summary bool, debug bool) {

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
			printAssetsDiff(b, oldAssets, newAssets, planning, indent+1, summary, debug)
			writeWithIndentNoPrefix(b, indent, deploy.OpUpdate, "}\n")
			return
		}
	}

	// Type of archive changed, print this out as an remove and an add.
	printDelete(
		b, assetOrArchiveToPropertyValue(oldArchive),
		titleFunc, planning, indent, debug)
	printAdd(
		b, assetOrArchiveToPropertyValue(newArchive),
		titleFunc, planning, indent, debug)
}

func printAssetsDiff(
	b *bytes.Buffer,
	oldAssets map[string]interface{}, newAssets map[string]interface{},
	planning bool, indent int, summary bool, debug bool) {

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

				old := oldAssets[oldName]
				new := newAssets[newName]

				// If the assets/archvies haven't changed, then don't bother printing them out.
				// This happens routinely when we have an archive that has changed because some
				// asset it in it changed.  We want *that* asset to be printed, but not all the
				// unchanged assets.

				switch t := old.(type) {
				case *resource.Archive:
					newArchive, newIsArchive := new.(*resource.Archive)
					switch {
					case !newIsArchive:
						printAssetArchiveDiff(b, titleFunc, t, new, planning, indent, summary, debug)
					case t.Hash != newArchive.Hash:
						printArchiveDiff(
							b, titleFunc, t, newArchive,
							planning, indent, summary, debug)
					}
				case *resource.Asset:
					newAsset, newIsAsset := new.(*resource.Asset)
					switch {
					case !newIsAsset:
						printAssetArchiveDiff(b, titleFunc, t, new, planning, indent, summary, debug)
					case t.Hash != newAsset.Hash:
						printAssetDiff(
							b, titleFunc, t, newAsset,
							planning, indent, summary, debug)
					}
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
			printDelete(
				b, assetOrArchiveToPropertyValue(oldAssets[oldName]),
				titleFunc, planning, newIndent, debug)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			titleFunc := func(top deploy.StepOp, tprefix bool) {
				printPropertyTitle(b, "\""+newName+"\"", maxkey, indent, top, tprefix)
			}
			printAdd(
				b, assetOrArchiveToPropertyValue(newAssets[newName]),
				titleFunc, planning, newIndent, debug)
			j++
		}
	}
}

func printAssetDiff(
	b *bytes.Buffer, titleFunc func(deploy.StepOp, bool),
	oldAsset *resource.Asset, newAsset *resource.Asset,
	planning bool, indent int, summary bool, debug bool) {

	contract.Assertf(oldAsset.Hash != newAsset.Hash, "Should not call printAssetDiff on unchanged assets")

	op := deploy.OpUpdate

	// if the asset changed, print out: ~ assetName: type(hash->hash) details...
	hashChange := getTextChangeString(shortHash(oldAsset.Hash), shortHash(newAsset.Hash))

	if oldAsset.IsText() {
		if newAsset.IsText() {
			titleFunc(deploy.OpUpdate, true)
			write(b, op, "asset(text:%s) {\n", hashChange)

			massagedOldText := resource.MassageIfUserProgramCodeAsset(oldAsset, debug).Text
			massagedNewText := resource.MassageIfUserProgramCodeAsset(newAsset, debug).Text

			differ := diffmatchpatch.New()
			differ.DiffTimeout = 0

			hashed1, hashed2, lineArray := differ.DiffLinesToChars(massagedOldText, massagedNewText)
			diffs1 := differ.DiffMain(hashed1, hashed2, false)
			diffs2 := differ.DiffCharsToLines(diffs1, lineArray)

			writeString(b, diffToPrettyString(diffs2, indent+1))

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
		titleFunc, planning, indent, debug)
	printAdd(
		b, assetOrArchiveToPropertyValue(newAsset),
		titleFunc, planning, indent, debug)
}

func printAssetArchiveDiff(b *bytes.Buffer, titleFunc func(deploy.StepOp, bool), old interface{}, new interface{},
	planning bool, indent int, summary bool, debug bool) {
	printDelete(b, assetOrArchiveToPropertyValue(old), titleFunc, planning, indent, debug)
	printAdd(b, assetOrArchiveToPropertyValue(new), titleFunc, planning, indent, debug)
}

func getTextChangeString(old string, new string) string {
	if old == new {
		return old
	}

	return fmt.Sprintf("%s->%s", old, new)
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

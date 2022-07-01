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

package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/sergi/go-diff/diffmatchpatch"
	"gopkg.in/yaml.v3"
)

// getIndent computes a step's parent indentation.
func getIndent(step engine.StepEventMetadata, seen map[resource.URN]engine.StepEventMetadata) int {
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

func printStepHeader(b io.StringWriter, step engine.StepEventMetadata) {
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

func getIndentationString(indent int, op display.StepOp, prefix bool) string {
	result := strings.Repeat("    ", indent)

	if !prefix {
		return result
	}

	if result == "" {
		contract.Assertf(!prefix, "Expected indention for a prefixed line")
		return result
	}

	rp := deploy.RawPrefix(op)
	contract.Assert(len(rp) == 2)
	contract.Assert(len(result) >= 2)
	return result[:len(result)-2] + rp
}

func writeString(b io.StringWriter, s string) {
	_, err := b.WriteString(s)
	contract.IgnoreError(err)
}

func writeWithIndent(b io.StringWriter, indent int, op display.StepOp, prefix bool, format string, a ...interface{}) {
	writeString(b, deploy.Color(op))
	writeString(b, getIndentationString(indent, op, prefix))
	writeString(b, fmt.Sprintf(format, a...))
	writeString(b, colors.Reset)
}

func writeWithIndentNoPrefix(b io.StringWriter, indent int, op display.StepOp, format string, a ...interface{}) {
	writeWithIndent(b, indent, op, false, format, a...)
}

func write(b io.StringWriter, op display.StepOp, format string, a ...interface{}) {
	writeWithIndentNoPrefix(b, 0, op, format, a...)
}

func writeVerbatim(b io.StringWriter, op display.StepOp, value string) {
	writeWithIndentNoPrefix(b, 0, op, "%s", value)
}

func getResourcePropertiesSummary(step engine.StepEventMetadata, indent int) string {
	var b bytes.Buffer

	op := step.Op
	urn := step.URN
	old := step.Old

	// Print the indentation.
	writeString(&b, getIndentationString(indent, op, false))

	// First, print out the operation's prefix.
	writeString(&b, deploy.Prefix(op, true /*done*/))

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

func getResourcePropertiesDetails(
	step engine.StepEventMetadata, indent int, planning bool, summary bool, fullOutput bool, debug bool) string {
	var b bytes.Buffer

	// indent everything an additional level, like other properties.
	indent++

	old, new := step.Old, step.New
	if old == nil && new != nil {
		if len(new.Outputs) > 0 {
			PrintObject(&b, new.Outputs, planning, indent, step.Op, false, fullOutput, debug)
		} else {
			PrintObject(&b, new.Inputs, planning, indent, step.Op, false, fullOutput, debug)
		}
	} else if new == nil && old != nil {
		// in summary view, we don't have to print out the entire object that is getting deleted.
		// note, the caller will have already printed out the type/name/id/urn of the resource,
		// and that's sufficient for a summarized deletion view.
		if !summary {
			PrintObject(&b, old.Inputs, planning, indent, step.Op, false, fullOutput, debug)
		}
	} else if len(new.Outputs) > 0 && step.Op != deploy.OpImport && step.Op != deploy.OpImportReplacement {
		printOldNewDiffs(&b, old.Outputs, new.Outputs, nil, planning, indent, step.Op, summary, fullOutput, debug)
	} else {
		printOldNewDiffs(&b, old.Inputs, new.Inputs, step.Diffs, planning, indent, step.Op, summary, fullOutput, debug)
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
	indent int, op display.StepOp, prefix bool, fullOutput bool, debug bool) {

	p := propertyPrinter{
		dest:       b,
		planning:   planning,
		indent:     indent,
		op:         op,
		prefix:     prefix,
		debug:      debug,
		fullOutput: fullOutput,
	}
	p.printObject(props)
}

func (p *propertyPrinter) printObject(props resource.PropertyMap) {
	// Compute the maximum width of property keys so we can justify everything.
	keys := props.StableKeys()
	maxkey := maxKey(keys)

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; !resource.IsInternalPropertyKey(k) && shouldPrintPropertyValue(v, p.planning) {
			p.printObjectProperty(k, v, maxkey)
		}
	}
}

func (p *propertyPrinter) printObjectProperty(key resource.PropertyKey, value resource.PropertyValue, maxkey int) {
	p.printPropertyTitle(string(key), maxkey)
	p.printPropertyValue(value)
}

func PrintResourceReference(
	b *bytes.Buffer, resRef resource.ResourceReference, planning bool,
	indent int, op display.StepOp, prefix bool, debug bool) {

	p := propertyPrinter{
		dest:     b,
		planning: planning,
		indent:   indent,
		op:       op,
		prefix:   prefix,
		debug:    debug,
	}
	p.printResourceReference(resRef)
}

func (p *propertyPrinter) printResourceReference(resRef resource.ResourceReference) {
	p.printPropertyTitle("URN", 3)
	p.write("%q\n", resRef.URN)
	p.printPropertyTitle("ID", 3)
	p.printPropertyValue(resRef.ID)
	p.printPropertyTitle("PackageVersion", 3)
	p.write("%q\n", resRef.PackageVersion)
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

// getResourceOutputsPropertiesString prints only those properties that either differ from the input properties or, if
// there is an old snapshot of the resource, differ from the prior old snapshot's output properties.
func getResourceOutputsPropertiesString(
	step engine.StepEventMetadata, indent int, planning, debug, refresh, showSames bool) string {

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

		// If we asked not to show-sames, and no outputs changed then don't show anything at all here.
		if outputDiff == nil && !showSames {
			return ""
		}
	}

	var keys []resource.PropertyKey
	if outputDiff == nil {
		keys = outs.StableKeys()
	} else {
		keys = outputDiff.Keys()
	}
	maxkey := maxKey(keys)

	b := &bytes.Buffer{}
	p := propertyPrinter{
		dest:     b,
		planning: planning,
		indent:   indent,
		op:       op,
		debug:    debug,
	}

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
				p.printObjectPropertyDiff(k, maxkey, *outputDiff)
			} else {
				p.printObjectProperty(k, out, maxkey)
			}
		}
	}

	return b.String()
}

func considerSameIfNotCreateOrDelete(op display.StepOp) display.StepOp {
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

type propertyPrinter struct {
	dest io.StringWriter

	op         display.StepOp
	planning   bool
	prefix     bool
	debug      bool
	summary    bool
	fullOutput bool

	indent int
}

func (p *propertyPrinter) indented(amt int) *propertyPrinter {
	new := *p
	new.indent += amt
	return &new
}

func (p *propertyPrinter) withOp(op display.StepOp) *propertyPrinter {
	new := *p
	new.op = op
	return &new
}

func (p *propertyPrinter) withPrefix(value bool) *propertyPrinter {
	new := *p
	new.prefix = value
	return &new
}

func (p *propertyPrinter) writeString(s string) {
	writeString(p.dest, s)
}

func (p *propertyPrinter) writeWithIndent(format string, a ...interface{}) {
	writeWithIndent(p.dest, p.indent, p.op, p.prefix, format, a...)
}

func (p *propertyPrinter) writeWithIndentNoPrefix(format string, a ...interface{}) {
	writeWithIndentNoPrefix(p.dest, p.indent, p.op, format, a...)
}

func (p *propertyPrinter) write(format string, a ...interface{}) {
	write(p.dest, p.op, format, a...)
}

func (p *propertyPrinter) writeVerbatim(value string) {
	writeVerbatim(p.dest, p.op, value)
}

func (p *propertyPrinter) printPropertyTitle(name string, align int) {
	p.writeWithIndent("%-"+strconv.Itoa(align)+"s: ", name)
}

func propertyTitlePrinter(name string, align int) func(*propertyPrinter) {
	return func(p *propertyPrinter) {
		p.printPropertyTitle(name, align)
	}
}

func (p *propertyPrinter) printPropertyValue(v resource.PropertyValue) {
	switch {
	case isPrimitive(v):
		p.printPrimitivePropertyValue(v)
	case v.IsArray():
		arr := v.ArrayValue()
		if len(arr) == 0 {
			p.writeVerbatim("[]")
		} else {
			p.writeVerbatim("[\n")
			for i, elem := range arr {
				p.writeWithIndent("    [%d]: ", i)
				p.indented(1).printPropertyValue(elem)
			}
			p.writeWithIndentNoPrefix("]")
		}
	case v.IsAsset():
		a := v.AssetValue()
		if a.IsText() {
			p.write("asset(text:%s) {\n", shortHash(a.Hash))

			a = resource.MassageIfUserProgramCodeAsset(a, p.debug)

			massaged := a.Text

			// pretty print the text, line by line, with proper breaks.
			lines := strings.Split(massaged, "\n")
			for _, line := range lines {
				p.writeWithIndentNoPrefix("    %s\n", line)
			}
			p.writeWithIndentNoPrefix("}")
		} else if path, has := a.GetPath(); has {
			p.write("asset(file:%s) { %s }", shortHash(a.Hash), path)
		} else {
			contract.Assert(a.IsURI())
			p.write("asset(uri:%s) { %s }", shortHash(a.Hash), a.URI)
		}
	case v.IsArchive():
		a := v.ArchiveValue()
		if assets, has := a.GetAssets(); has {
			p.write("archive(assets:%s) {\n", shortHash(a.Hash))
			var names []string
			for name := range assets {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				p.printAssetOrArchive(assets[name], name)
			}
			p.writeWithIndentNoPrefix("}")
		} else if path, has := a.GetPath(); has {
			p.write("archive(file:%s) { %s }", shortHash(a.Hash), path)
		} else {
			contract.Assert(a.IsURI())
			p.write("archive(uri:%s) { %v }", shortHash(a.Hash), a.URI)
		}
	case v.IsObject():
		obj := v.ObjectValue()
		if len(obj) == 0 {
			p.writeVerbatim("{}")
		} else {
			p.writeVerbatim("{\n")
			p.indented(1).printObject(obj)
			p.writeWithIndentNoPrefix("}")
		}
	case v.IsResourceReference():
		resRef := v.ResourceReferenceValue()
		p.writeVerbatim("{\n")
		p.indented(1).printResourceReference(resRef)
		p.writeWithIndentNoPrefix("}")
	default:
		contract.Failf("Unknown PropertyValue type %v", v)
	}
	p.writeVerbatim("\n")
}

func (p *propertyPrinter) printAssetOrArchive(v interface{}, name string) {
	p.writeWithIndent("    \"%v\": ", name)
	p.indented(1).printPropertyValue(assetOrArchiveToPropertyValue(v))
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
	planning bool, indent int, op display.StepOp, summary bool, fullOutput bool, debug bool) {

	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news, resource.IsInternalPropertyKey); diff != nil {
		PrintObjectDiff(b, *diff, include, planning, indent, summary, fullOutput, debug)
	} else {
		// If there's no diff, report the op as Same - there's no diff to render
		// so it should be rendered as if nothing changed.
		PrintObject(b, news, planning, indent, deploy.OpSame, true, fullOutput, debug)
	}
}

func PrintObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff, include []resource.PropertyKey,
	planning bool, indent int, summary bool, fullOutput bool, debug bool) {

	p := propertyPrinter{
		dest:       b,
		planning:   planning,
		indent:     indent,
		prefix:     true,
		debug:      debug,
		summary:    summary,
		fullOutput: fullOutput,
	}
	p.printObjectDiff(diff, include)
}

func (p *propertyPrinter) printObjectDiff(diff resource.ObjectDiff, include []resource.PropertyKey) {
	contract.Assert(p.indent > 0)

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
		p.printObjectPropertyDiff(k, maxkey, diff)
	}
}

func (p *propertyPrinter) printObjectPropertyDiff(key resource.PropertyKey, maxkey int, diff resource.ObjectDiff) {
	titleFunc := propertyTitlePrinter(string(key), maxkey)
	if add, isadd := diff.Adds[key]; isadd {
		p.printAdd(add, titleFunc)
	} else if delete, isdelete := diff.Deletes[key]; isdelete {
		p.printDelete(delete, titleFunc)
	} else if update, isupdate := diff.Updates[key]; isupdate {
		p.printPropertyValueDiff(titleFunc, update)
	} else if same := diff.Sames[key]; !p.summary && shouldPrintPropertyValue(same, p.planning) {
		p.withOp(deploy.OpSame).withPrefix(false).printObjectProperty(key, same, maxkey)
	}
}

func (p *propertyPrinter) printPropertyValueDiff(titleFunc func(*propertyPrinter), diff resource.ValueDiff) {
	p = p.withOp(deploy.OpUpdate).withPrefix(true)
	contract.Assert(p.indent > 0)

	if diff.Array != nil {
		titleFunc(p)
		p.writeVerbatim("[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			elemPrinter := p.indented(2)
			elemTitleFunc := func(p *propertyPrinter) {
				p.indented(-1).writeWithIndent("[%d]: ", i)
			}

			if add, isadd := a.Adds[i]; isadd {
				elemPrinter.printAdd(add, elemTitleFunc)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				elemPrinter.printDelete(delete, elemTitleFunc)
			} else if update, isupdate := a.Updates[i]; isupdate {
				elemPrinter.printPropertyValueDiff(elemTitleFunc, update)
			} else if same, issame := a.Sames[i]; issame && !p.summary {
				elemPrinter = elemPrinter.withOp(deploy.OpSame).withPrefix(false)
				elemTitleFunc(elemPrinter)
				elemPrinter.printPropertyValue(same)
			}
		}
		p.writeWithIndentNoPrefix("]\n")
	} else if diff.Object != nil {
		titleFunc(p)
		p.writeVerbatim("{\n")
		p.indented(1).printObjectDiff(*diff.Object, nil)
		p.writeWithIndentNoPrefix("}\n")
	} else {
		shouldPrintOld := shouldPrintPropertyValue(diff.Old, false)
		shouldPrintNew := shouldPrintPropertyValue(diff.New, false)

		if shouldPrintOld && shouldPrintNew {
			if diff.Old.IsArchive() &&
				diff.New.IsArchive() {

				p.printArchiveDiff(titleFunc, diff.Old.ArchiveValue(), diff.New.ArchiveValue())
				return
			}

			if isPrimitive(diff.Old) && isPrimitive(diff.New) {
				titleFunc(p)

				if diff.Old.IsString() && diff.New.IsString() {
					p.printTextDiff(diff.Old.StringValue(), diff.New.StringValue())
					return
				}

				p.withOp(deploy.OpDelete).printPrimitivePropertyValue(diff.Old)
				p.writeVerbatim(" => ")
				p.withOp(deploy.OpCreate).printPrimitivePropertyValue(diff.New)
				p.writeVerbatim("\n")
				return
			}
		}

		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintOld {
			p.printDelete(diff.Old, titleFunc)
		}
		if shouldPrintNew {
			p.printAdd(diff.New, titleFunc)
		}
	}
}

func isPrimitive(value resource.PropertyValue) bool {
	return value.IsNull() || value.IsString() || value.IsNumber() ||
		value.IsBool() || value.IsComputed() || value.IsOutput() || value.IsSecret()
}

func (p *propertyPrinter) printPrimitivePropertyValue(v resource.PropertyValue) {
	contract.Assert(isPrimitive(v))
	if v.IsNull() {
		p.writeVerbatim("<null>")
	} else if v.IsBool() {
		p.write("%t", v.BoolValue())
	} else if v.IsNumber() {
		p.write("%v", v.NumberValue())
	} else if v.IsString() {
		if vv, kind, ok := p.decodeValue(v.StringValue()); ok {
			p.write("(%s) ", kind)
			p.printPropertyValue(vv)
			return
		}
		if p.fullOutput {
			p.write("%q", v.StringValue())
		} else {
			p.write("%q", p.truncatePropertyString(v.StringValue()))
		}
	} else if v.IsComputed() || v.IsOutput() {
		// We render computed and output values differently depending on whether or not we are
		// planning or deploying: in the former case, we display `computed<type>` or `output<type>`;
		// in the former we display `undefined`. This is because we currently cannot distinguish
		// between user-supplied undefined values and input properties that are undefined because
		// they were sourced from undefined values in other resources' output properties. Once we
		// have richer information about the dataflow between resources, we should be able to do a
		// better job here (pulumi/pulumi#234).
		if p.planning {
			p.writeVerbatim(v.TypeString())
		} else {
			p.write("undefined")
		}
	} else if v.IsSecret() {
		p.write("[secret]")
	} else {
		contract.Failf("Unexpected property value kind '%v'", v)
	}
}

func (p *propertyPrinter) printDelete(v resource.PropertyValue, title func(*propertyPrinter)) {
	p = p.withOp(deploy.OpDelete).withPrefix(true)
	title(p)
	p.printPropertyValue(v)
}

func (p *propertyPrinter) printAdd(v resource.PropertyValue, title func(*propertyPrinter)) {
	p = p.withOp(deploy.OpCreate).withPrefix(true)
	title(p)
	p.printPropertyValue(v)
}

func (p *propertyPrinter) printArchiveDiff(titleFunc func(*propertyPrinter),
	oldArchive, newArchive *resource.Archive) {

	p = p.withOp(deploy.OpUpdate).withPrefix(true)

	hashChange := getTextChangeString(shortHash(oldArchive.Hash), shortHash(newArchive.Hash))

	if oldPath, has := oldArchive.GetPath(); has {
		if newPath, has := newArchive.GetPath(); has {
			titleFunc(p)
			p.write("archive(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else if oldURI, has := oldArchive.GetURI(); has {
		if newURI, has := newArchive.GetURI(); has {
			titleFunc(p)
			p.write("archive(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	} else {
		contract.Assert(oldArchive.IsAssets())
		oldAssets, _ := oldArchive.GetAssets()

		if newAssets, has := newArchive.GetAssets(); has {
			titleFunc(p)
			p.write("archive(assets:%s) {\n", hashChange)
			p.indented(1).printAssetsDiff(oldAssets, newAssets)
			p.writeWithIndentNoPrefix("}\n")
			return
		}
	}

	// Type of archive changed, print this out as an remove and an add.
	p.printDelete(assetOrArchiveToPropertyValue(oldArchive), titleFunc)
	p.printAdd(assetOrArchiveToPropertyValue(newArchive), titleFunc)
}

func (p *propertyPrinter) printAssetsDiff(oldAssets, newAssets map[string]interface{}) {
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
				titleFunc := propertyTitlePrinter("\""+oldName+"\"", maxkey)

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
						p.printAssetArchiveDiff(titleFunc, t, new)
					case t.Hash != newArchive.Hash:
						p.printArchiveDiff(titleFunc, t, newArchive)
					}
				case *resource.Asset:
					newAsset, newIsAsset := new.(*resource.Asset)
					switch {
					case !newIsAsset:
						p.printAssetArchiveDiff(titleFunc, t, new)
					case t.Hash != newAsset.Hash:
						p.printAssetDiff(titleFunc, t, newAsset)
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

		if deleteOld {
			oldName := oldNames[i]
			titleFunc := propertyTitlePrinter("\""+oldName+"\"", maxkey)
			p.indented(1).printDelete(assetOrArchiveToPropertyValue(oldAssets[oldName]), titleFunc)
			i++
			continue
		} else {
			contract.Assert(addNew)
			newName := newNames[j]
			titleFunc := propertyTitlePrinter("\""+newName+"\"", maxkey)
			p.indented(1).printAdd(assetOrArchiveToPropertyValue(newAssets[newName]), titleFunc)
			j++
		}
	}
}

func (p *propertyPrinter) printAssetDiff(titleFunc func(*propertyPrinter), oldAsset, newAsset *resource.Asset) {
	contract.Assertf(oldAsset.Hash != newAsset.Hash, "Should not call printAssetDiff on unchanged assets")

	p = p.withOp(deploy.OpUpdate).withPrefix(true)

	// if the asset changed, print out: ~ assetName: type(hash->hash) details...
	hashChange := getTextChangeString(shortHash(oldAsset.Hash), shortHash(newAsset.Hash))

	if oldAsset.IsText() {
		if newAsset.IsText() {
			titleFunc(p)
			p.write("asset(text:%s) {", hashChange)

			massagedOldText := resource.MassageIfUserProgramCodeAsset(oldAsset, p.debug).Text
			massagedNewText := resource.MassageIfUserProgramCodeAsset(newAsset, p.debug).Text

			p.indented(1).printTextDiff(massagedOldText, massagedNewText)

			p.writeWithIndentNoPrefix("}\n")
			return
		}
	} else if oldPath, has := oldAsset.GetPath(); has {
		if newPath, has := newAsset.GetPath(); has {
			titleFunc(p)
			p.write("asset(file:%s) { %s }\n", hashChange, getTextChangeString(oldPath, newPath))
			return
		}
	} else {
		contract.Assert(oldAsset.IsURI())

		oldURI, _ := oldAsset.GetURI()
		if newURI, has := newAsset.GetURI(); has {
			titleFunc(p)
			p.write("asset(uri:%s) { %s }\n", hashChange, getTextChangeString(oldURI, newURI))
			return
		}
	}

	// Type of asset changed, print this out as an remove and an add.
	p.printDelete(assetOrArchiveToPropertyValue(oldAsset), titleFunc)
	p.printAdd(assetOrArchiveToPropertyValue(newAsset), titleFunc)
}

func (p *propertyPrinter) printAssetArchiveDiff(titleFunc func(p *propertyPrinter), old, new interface{}) {
	p.printDelete(assetOrArchiveToPropertyValue(old), titleFunc)
	p.printAdd(assetOrArchiveToPropertyValue(new), titleFunc)
}

func getTextChangeString(old string, new string) string {
	if old == new {
		return old
	}

	return fmt.Sprintf("%s->%s", old, new)
}

func escape(s string) string {
	escaped := strconv.Quote(s)
	return escaped[1 : len(escaped)-1]
}

func (p *propertyPrinter) printTextDiff(old, new string) {
	if p.printEncodedValueDiff(old, new) {
		return
	}

	differ := diffmatchpatch.New()
	differ.DiffTimeout = 0

	singleLine := !strings.ContainsRune(old, '\n') && !strings.ContainsRune(new, '\n')
	if singleLine {
		diff := differ.DiffMain(old, new, false)
		p.printCharacterDiff(differ.DiffCleanupEfficiency(diff))
	} else {
		hashed1, hashed2, lineArray := differ.DiffLinesToChars(old, new)
		diffs := differ.DiffMain(hashed1, hashed2, false)
		p.indented(1).printLineDiff(differ.DiffCharsToLines(diffs, lineArray))
	}
}

func (p *propertyPrinter) printCharacterDiff(diffs []diffmatchpatch.Diff) {
	// write the old text.
	p.writeVerbatim(`"`)
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			p.withOp(deploy.OpDelete).write(escape(d.Text))
		case diffmatchpatch.DiffEqual:
			p.withOp(deploy.OpSame).write(escape(d.Text))
		}
	}
	p.writeVerbatim(`"`)

	p.writeVerbatim(" => ")

	// write the new text.
	p.writeVerbatim(`"`)
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			p.withOp(deploy.OpCreate).write(escape(d.Text))
		case diffmatchpatch.DiffEqual:
			p.withOp(deploy.OpSame).write(escape(d.Text))
		}
	}
	p.writeVerbatim("\"\n")
}

// printLineDiff takes the full diff produed by diffmatchpatch and condenses it into something
// useful we can print to the console. Specifically, while it includes any adds/removes in
// green/red, it will also show portions of the unchanged text to help give surrounding context to
// those add/removes. Because the unchanged portions may be very large, it only included around 3
// lines before/after the change.
func (p *propertyPrinter) printLineDiff(diffs []diffmatchpatch.Diff) {
	p.writeVerbatim("\n")

	writeDiff := func(op display.StepOp, text string) {
		prefix := op == deploy.OpCreate || op == deploy.OpDelete
		p.withOp(op).withPrefix(prefix).writeWithIndent("%s", text)
	}

	for index, diff := range diffs {
		text := diff.Text
		lines := strings.Split(text, "\n")
		printLines := func(op display.StepOp, startInclusive int, endExclusive int) {
			for i := startInclusive; i < endExclusive; i++ {
				if strings.TrimSpace(lines[i]) != "" {
					writeDiff(op, lines[i])
					p.writeString("\n")
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
}

func (p *propertyPrinter) printEncodedValueDiff(old, new string) bool {
	oldValue, oldKind, ok := p.decodeValue(old)
	if !ok {
		return false
	}

	newValue, newKind, ok := p.decodeValue(new)
	if !ok {
		return false
	}

	if oldKind == newKind {
		p.write("(%s) ", oldKind)
	} else {
		p.write("(%s => %s) ", oldKind, newKind)
	}

	diff := oldValue.Diff(newValue, resource.IsInternalPropertyKey)
	if diff == nil {
		p.withOp(deploy.OpSame).printPropertyValue(oldValue)
		return true
	}

	p.printPropertyValueDiff(func(*propertyPrinter) {}, *diff)
	return true
}

func (p *propertyPrinter) decodeValue(repr string) (resource.PropertyValue, string, bool) {
	decode := func() (interface{}, string, bool) {
		r := strings.NewReader(repr)

		var object interface{}
		if err := json.NewDecoder(r).Decode(&object); err == nil {
			return object, "json", true
		}

		r.Reset(repr)
		if err := yaml.NewDecoder(r).Decode(&object); err == nil {
			translated, ok := p.translateYAMLValue(object)
			if !ok {
				return nil, "", false
			}
			return translated, "yaml", true
		}

		return nil, "", false
	}

	object, kind, ok := decode()
	if ok {
		switch object.(type) {
		case []interface{}, map[string]interface{}:
			return resource.NewPropertyValue(object), kind, true
		}
	}
	return resource.PropertyValue{}, "", false
}

// translateYAMLValue attempts to replace map[interface{}]interface{} values in a decoded YAML value with
// map[string]interface{} values. map[interface{}]interface{} values can arise from YAML mappings with keys that are
// not strings. This method only translates such maps if they have purely numeric keys--maps with slice or map keys
// are not translated.
func (p *propertyPrinter) translateYAMLValue(v interface{}) (interface{}, bool) {
	switch v := v.(type) {
	case []interface{}:
		for i, e := range v {
			ee, ok := p.translateYAMLValue(e)
			if !ok {
				return nil, false
			}
			v[i] = ee
		}
		return v, true
	case map[string]interface{}:
		for k, e := range v {
			ee, ok := p.translateYAMLValue(e)
			if !ok {
				return nil, false
			}
			v[k] = ee
		}
		return v, true
	case map[interface{}]interface{}:
		vv := make(map[string]interface{}, len(v))
		for k, e := range v {
			sk := ""
			switch k := k.(type) {
			case string:
				sk = k
			case int:
				sk = strconv.FormatInt(int64(k), 10)
			case int64:
				sk = strconv.FormatInt(k, 10)
			case uint64:
				sk = strconv.FormatUint(k, 10)
			case float64:
				sk = strconv.FormatFloat(k, 'g', -1, 64)
			default:
				return nil, false
			}

			ee, ok := p.translateYAMLValue(e)
			if !ok {
				return nil, false
			}
			vv[sk] = ee
		}
		return vv, true
	default:
		return v, true
	}
}

// if string exceeds three lines, truncate and add "..."
func (p *propertyPrinter) truncatePropertyString(propertyString string) string {
	const contextLines = 3

	lines := strings.Split(propertyString, "\n")
	if len(lines) > contextLines {
		return strings.Join(lines[:3], "\n") + "..."
	}
	return propertyString
}

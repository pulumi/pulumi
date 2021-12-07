// Copyright 2016-2021, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const MaxPageSize int = 20
const MaxLineSize int = 80

func newSchemaExamineCommand() *cobra.Command {
	var displayEmpty bool
	cmd := &cobra.Command{
		Use:   "examine",
		Args:  cmdutil.ExactArgs(1),
		Short: "Interactively examine a Pulumi package schema",
		Long:  "Interactively examine a Pulumi package schema.\n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			pkgSpec, err := readSchemaFromFile(args[0])
			if err != nil {
				return err
			}
			pkg, diags, err := schema.BindSpec(*pkgSpec, nil)
			diagWriter := hcl.NewDiagnosticTextWriter(os.Stderr, nil, 0, true)
			wrErr := diagWriter.WriteDiagnostics(diags)
			contract.IgnoreError(wrErr)
			if err != nil {
				return fmt.Errorf("failed to bind schema: %w", err)
			}
			if err == nil && diags.HasErrors() {
				return fmt.Errorf("cannot examine invalid schema")
			}

			opts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			}

			return displaySchema(pkg, opts, displayEmpty)
		}),
	}
	cmd.PersistentFlags().BoolVarP(
		&displayEmpty, "display-empty", "d", false,
		"Show empty struct fields")

	return cmd
}

type selectFunc = func(string, reflect.Value, displayArgs)

func selectFunction(v reflect.Value) selectFunc {
	switch v.Kind() {
	case reflect.Struct:
		return selectStruct
	case reflect.Map:
		return selectMap
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		return selectSlice
	case reflect.String:
		return showString
	case reflect.Ptr:
		fallthrough
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return selectFunction(v.Elem())

	default:
		fmt.Printf("Could not provide function for: %s\n", v.Kind().String())
		return nil
	}
}

type displayArgs struct {
	isTopLevel   bool
	displayEmpty bool
}

func (a *displayArgs) NotTopLevel() displayArgs {
	t := *a
	t.isTopLevel = false
	return t
}

func displaySchema(spec *schema.Package, opts display.Options, displayEmpty bool) error {
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	prompt := "Schema"
	prompt = opts.Color.Colorize(colors.SpecPrompt + prompt + colors.Reset)
	structValue := *spec
	args := displayArgs{
		isTopLevel:   true,
		displayEmpty: displayEmpty,
	}
	selectStruct(prompt, reflect.ValueOf(structValue), args)
	return nil
}

func selectStruct(prompt string, v reflect.Value, args displayArgs) {
	v = drillType(v)
	contract.Assert(v.Kind() == reflect.Struct)
	fields := reflect.VisibleFields(v.Type())
	displayFields := []string{}
	displayValues := []reflect.StructField{}
	maxFieldLength := 0
	for _, f := range fields {
		if !f.IsExported() {
			continue
		}
		if !args.displayEmpty && isEmptyValue(v.FieldByIndex(f.Index)) {
			continue
		}

		displayValues = append(displayValues, f)
		displayFields = append(displayFields, f.Name)
		if len(f.Name) > maxFieldLength {
			maxFieldLength = len(f.Name)
		}
	}
	displayOptions := make([]string, len(displayFields))
	for i := range displayOptions {
		f := displayFields[i]
		v := displayValue(v.FieldByIndex(displayValues[i].Index))
		displayOptions[i] = fmt.Sprintf("%-*s: %s", maxFieldLength, f, v)
	}

	showRepeatedPrompt(prompt, displayOptions, args.isTopLevel, func(index int) {
		nextField := displayValues[index]
		nextValue := drillType(v.FieldByIndex(nextField.Index))
		nextFunc := selectFunction(nextValue)
		if nextFunc == nil {
			return
		}
		nextFunc(prompt+"."+nextField.Name, nextValue, args.NotTopLevel())
	})
}

func selectSlice(prompt string, slice reflect.Value, args displayArgs) {
	slice = drillType(slice)
	contract.Assertf(slice.Kind() == reflect.Slice ||
		slice.Kind() == reflect.Array, "Found type %s", slice.Kind().String())
	maxLength := int(math.Log10(float64(slice.Len())))
	displayOptions := make([]string, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		v := displayValue(slice.Index(i))
		displayOptions[i] = fmt.Sprintf("%-*d: %s", maxLength, i, v)
	}
	showRepeatedPrompt(prompt, displayOptions, false, func(index int) {
		nextValue := drillType(reflect.Indirect(slice.Index(index)))
		nextFunc := selectFunction(nextValue)
		if nextFunc == nil {
			return
		}
		nextFunc(prompt+fmt.Sprintf("[%d]", index), slice.Index(index), args)
	})
}

// We show the string broken up into lines.
func showString(prompt string, val reflect.Value, args displayArgs) {
	val = drillType(val)
	contract.Assert(val.Kind() == reflect.String)
	lines := breakStringIntoLines(val.String(), MaxLineSize)

	var result string
	noneDrillable := func(_ interface{}) error { return nil }
	pageSize := len(lines)
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	err := survey.AskOne(&survey.Select{
		Help:     "(select any line to go back)",
		Message:  fmt.Sprintf("%s", prompt),
		Options:  lines,
		PageSize: pageSize,
	}, &result, noneDrillable)
	cmdutil.MoveUp(1)

	contract.IgnoreError(err)
}

func selectMap(prompt string, mapValue reflect.Value, args displayArgs) {
	mapValue = drillType(mapValue)
	contract.Assert(mapValue.Kind() == reflect.Map)
	mapEntries := make([]struct {
		key   reflect.Value
		value reflect.Value
	}, mapValue.Len())
	iter := mapValue.MapRange()
	i := 0
	for iter.Next() {
		mapEntries[i].key = iter.Key()
		mapEntries[i].value = iter.Value()
		i++
	}
	sort.Slice(mapEntries, func(i int, j int) bool {
		return mapEntries[i].key.String() < mapEntries[j].key.String()
	})
	maxKeyLength := 0
	displayOptions := make([]string, len(mapEntries))
	for i, m := range mapEntries {
		f := displayValue(m.key)
		if len(f) > maxKeyLength {
			maxKeyLength = len(f)
		}
		displayOptions[i] = f
	}
	for i, k := range displayOptions {
		v := displayValue(mapEntries[i].value)
		displayOptions[i] = fmt.Sprintf("%-*s: %s", maxKeyLength, k, v)
	}

	showRepeatedPrompt(prompt, displayOptions, false, func(index int) {
		value := drillType(mapEntries[index].value)
		nextFunc := selectFunction(value)
		key := displayValue(mapEntries[index].key)
		if nextFunc == nil {
			return
		}
		nextFunc(prompt+fmt.Sprintf("[%s]", key), value, args)
	})
}

func displayValue(v reflect.Value) string {
	var arrayLen string
	switch v.Type().Kind() {
	case reflect.String:
		suffix := ""
		actual := v.String()
		if actual == "" {
			return "empty string"
		}
		max := MaxLineSize
		if len(actual) > max {
			suffix = "..."
		} else {
			max = len(actual)
		}
		return fmt.Sprintf("'%s'%s", actual[:max], suffix)

	// Numbers
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Uint:
		actual := v.Uint()
		return fmt.Sprintf("%d", actual)
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Int:
		actual := v.Int()
		return fmt.Sprintf("%d", actual)
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		actual := v.Float()
		return fmt.Sprintf("%f", actual)

	case reflect.Bool:
		actual := v.Bool()
		return fmt.Sprintf("%t", actual)

	case reflect.Array:
		arrayLen = fmt.Sprintf("%d", v.Len())
		fallthrough
	case reflect.Slice:
		value := v.Type().Elem().String()
		length := v.Len()
		endTag := fmt.Sprintf("(%d)", length)
		if length == 1 {
			endTag = fmt.Sprintf("[ %s ]", displayValue(v.Index(0)))
		}
		return fmt.Sprintf("[%s]%s %s", arrayLen, value, endTag)

	case reflect.Map:
		key := v.Type().Key().String()
		value := v.Type().Elem().String()
		length := len(v.MapKeys())
		endTag := fmt.Sprintf("(%d)", length)
		if length == 1 {
			iter := v.MapRange()
			contract.Assert(iter.Next())
			onlyKey := iter.Key()
			onlyValue := iter.Value()
			endTag = fmt.Sprintf("{ %s: %s }", displayValue(onlyKey), displayValue(onlyValue))
		}
		return fmt.Sprintf("map[%s]%s %s", key, value, endTag)

	case reflect.Struct:
		// special case version
		if version, ok := v.Interface().(semver.Version); ok {
			return version.String()
		}
		// If there is a `Name` or `Token` tag, display that next to the struct
		title := ""
		visibleFields := reflect.VisibleFields(v.Type())
		if len(visibleFields) == 1 {
			f := v.FieldByIndex(visibleFields[0].Index)
			title = fmt.Sprintf("(%s:%s)", visibleFields[0].Name, displayValue(f))
		} else {
			for _, f := range visibleFields {
				if f.Name == "Name" {
					title = fmt.Sprintf("(Name:%s)", displayValue(v.FieldByIndex(f.Index)))
					break
				}
				if f.Name == "Token" {
					title = fmt.Sprintf("(Token:%s)", displayValue(v.FieldByIndex(f.Index)))
				}
				if f.Name == "Value" {
					title = fmt.Sprintf("(Value:%s)", displayValue(v.FieldByIndex(f.Index)))
				}
			}
		}

		if title != "" {
			title = fmt.Sprintf(": %s", title)
		}
		typ := v.Type().String()
		return fmt.Sprintf("struct %s%s", typ, title)

	// Indirect types
	case reflect.Ptr:
		if v.IsNil() {
			return fmt.Sprintf("%s (nil)", v.Type().String())
		}
		return fmt.Sprintf("*%s", displayValue(v.Elem()))
	case reflect.Interface:
		if v.IsNil() {
			return fmt.Sprintf("interface{} (nil)")
		}
		v = drillType(v)

		return displayValue(v)

	default:
		return "unknown kind: " + v.Type().Kind().String()
	}
}

// Displays a prompt to the user, returning the index of the selected value.
//
// If a non-negative number is returned, then it is the index of the selected value.
// If -1 is returned, the user did not select a value.
// If -2 is returned, the user selected to leave.
func showExaminePrompt(prompt string, values []string, isExit bool) int {
	var leaveValue string
	if isExit {
		leaveValue = "(exit)"
	} else {
		leaveValue = "(back)"
	}

	isDrillable := func(ans interface{}) error {
		selected, ok := ans.(string)
		if !ok {
			return fmt.Errorf("Could not process answer: %v", ans)
		}

		if selected == leaveValue {
			return nil
		}

		// Is a collapsed string
		if strings.HasSuffix(selected, "...") {
			return nil
		}

		var typeTag string
		if i := strings.Index(selected, ":"); i > -1 {
			typeTag = strings.TrimSpace(selected[i+1:])
			typeTag = strings.TrimPrefix(typeTag, "*")
		} else {
			return fmt.Errorf("Could not find determine type for %q", selected)
		}
		if strings.HasPrefix(typeTag, "struct") {
			return nil
		} else if strings.HasPrefix(typeTag, "map") ||
			strings.HasPrefix(typeTag, "[]") {
			if strings.HasSuffix(typeTag, "(0)") {
				return fmt.Errorf("Cannot look deeper into empty containers")
			}
			return nil
		}
		// Fixup for display purposes
		if strings.HasPrefix(typeTag, "'") {
			typeTag = "string"
		}
		return fmt.Errorf("found type %s, only structs, maps or arrays are examinable", typeTag)
	}
	var result string
	options := append(values, leaveValue)
	pageSize := len(options)
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	err := survey.AskOne(&survey.Select{
		Message:  prompt,
		Options:  options,
		PageSize: pageSize,
	}, &result, isDrillable)

	if err != nil {
		return -1
	}
	if result == leaveValue {
		return -2
	}
	for i, v := range values {
		if v == result {
			return i
		}
	}
	return -1

}

func showRepeatedPrompt(prompt string, displayOptions []string, isTopLevel bool, drill func(i int)) {
	for {
		index := showExaminePrompt(prompt, displayOptions, isTopLevel)

		if index == -1 {
			// This will leave whatever is on the screen in place.
			os.Exit(0)
		}
		cmdutil.MoveUp(1)
		if index == -2 {
			return
		}

		drill(index)
	}
}

func drillType(v reflect.Value) reflect.Value {
	for {

		switch v.Kind() {
		case reflect.Ptr:
			if v.IsNil() {
				return v
			}
			v = v.Elem()
		case reflect.Interface:
			if v.IsNil() {
				return v
			}

			// Handle interface{} -> json.RawMessage
			if data, ok := v.Elem().Interface().(json.RawMessage); ok {
				jsonMap := map[string]interface{}{}
				if err := json.Unmarshal(data, &jsonMap); err == nil {
					return reflect.ValueOf(jsonMap)
				}
				jsonArray := []interface{}{}
				if err := json.Unmarshal(data, &jsonArray); err == nil {
					return reflect.ValueOf(jsonArray)
				}
				return reflect.ValueOf(string(data))
			}

			v = v.Elem()
		case reflect.Slice:
			var u8 []uint8 // This is what generic interfaces coerce to.
			if v.Type() == reflect.TypeOf(u8) {
				mapValue, ok := v.Interface().(map[string]interface{})
				if ok {
					return reflect.ValueOf(mapValue)
				}
			}
			fallthrough
		default:
			return v
		}
	}
}

func isEmptyValue(v reflect.Value) bool {
	v = drillType(v)
	switch v.Kind() {
	case reflect.Interface:
		fallthrough
	case reflect.Ptr:
		// We would have drilled if this had something in it
		return true
	case reflect.Map:
		fallthrough
	case reflect.Array:
		fallthrough
	case reflect.String:
		fallthrough
	case reflect.Slice:
		return v.Len() == 0
	default:
		return false
	}
}

// Attempts to break s into lines of length targetLength. This is a best effort, and does not
// guarantee that each line is less then targetLength.
func breakStringIntoLines(input string, targetLength int) []string {
	s := []rune(input)
	lines := []string{}
	for len(s) > targetLength {
		guess := targetLength

		// We find the end of the last word
		for !unicode.IsSpace(s[guess]) && guess > 0 {
			guess--
		}

		// The word is longer then targetLength. Take the whole word.
		if guess == 0 {
			guess = targetLength
			for guess < len(s) && !unicode.IsSpace(s[guess]) {
				guess++
			}
		}

		lines = append(lines, string(s[:guess]))
		s = s[guess:]
	}

	// Get the last line
	lines = append(lines, string(s))

	return lines
}

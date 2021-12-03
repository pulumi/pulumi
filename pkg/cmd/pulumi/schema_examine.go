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
	"fmt"
	"os"
	"path"
	"reflect"
	"sort"

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

func newSchemaExamineCommand() *cobra.Command {
	return &cobra.Command{
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
				return fmt.Errorf("schema validation failed")
			}

			opts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: true,
			}

			return displaySchema(pkg, opts)
		}),
	}
}

func isDrillable(ans interface{}) error {
	return nil
}

type selectFunc = func(string, reflect.Value)

func selectFunction(k reflect.Kind) selectFunc {
	switch k {
	case reflect.Struct:
		return selectStruct
	case reflect.Map:
		return selectMap
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		panic("Slices are not implemented")
	default:
		return nil
	}
}

func displaySchema(spec *schema.Package, opts display.Options) error {
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	prompt := "Schema"
	prompt = opts.Color.Colorize(colors.SpecPrompt + prompt + colors.Reset)
	structValue := *spec
	selectStruct(prompt, reflect.ValueOf(structValue))
	return nil
}

func selectStruct(prompt string, v reflect.Value) {
	contract.Assert(v.Kind() == reflect.Struct)
	fields := reflect.VisibleFields(v.Type())
	displayFields := []string{}
	displayValues := []reflect.StructField{}
	maxFieldLength := 0
	for _, f := range fields {
		if !f.IsExported() {
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

	var selectedField string
	survey.AskOne(&survey.Select{
		Message:  prompt,
		Options:  displayOptions,
		PageSize: len(displayOptions),
	}, &selectedField, isDrillable)
	index := -1
	for i, v := range displayOptions {
		if selectedField == v {
			index = i
			break
		}
	}
	contract.Assert(index != -1)
	nextField := displayValues[index]
	nextValue := v.FieldByIndex(nextField.Index)
	nextFunc := selectFunction(nextValue.Kind())
	if nextFunc == nil {
		return
	}
	nextFunc(path.Join(prompt, nextField.Name), nextValue)
}

func selectMap(prompt string, mapValue reflect.Value) {
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
		fmt.Sprintf("%-*s: %s", maxKeyLength, k, v)
	}
	var result string
	survey.AskOne(&survey.Select{
		Message:  prompt,
		Options:  displayOptions,
		PageSize: len(displayOptions),
	}, &result, isDrillable)
	index := -1
	for i, v := range displayOptions {
		if result == v {
			index = i
			break
		}
	}
	contract.Assert(index > -1)
	// index into map
}

func displayValue(v reflect.Value) string {
	switch v.Type().Kind() {
	case reflect.String:
		suffix := ""
		actual := v.String()
		if actual == "" {
			return "empty string"
		} else {
			max := 80
			if len(actual) > max {
				suffix = "..."
			} else {
				max = len(actual)
			}
			return fmt.Sprintf("'%s'%s", actual[:max], suffix)
		}
	case reflect.Int:
		actual := v.Int()
		return fmt.Sprintf("%d", actual)
	case reflect.Bool:
		actual := v.Bool()
		return fmt.Sprintf("%t", actual)
	case reflect.Slice:
		fallthrough
	case reflect.Array:
		value := v.Type().Elem().String()
		length := v.Len()
		return fmt.Sprintf("[]%s (%d)", value, length)
	case reflect.Map:
		key := v.Type().Key().String()
		value := v.Type().Elem().String()
		length := len(v.MapKeys())
		return fmt.Sprintf("map[%s]%s (%d)", key, value, length)
	case reflect.Struct:
		// special case version
		if version, ok := v.Interface().(semver.Version); ok {
			return version.String()
		}
		typ := v.Type().String()
		return fmt.Sprintf("struct %s", typ)
	case reflect.Ptr:
		if v.IsNil() {
			return fmt.Sprintf("%s (nil)", v.Type().String())
		}
		return displayValue(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return fmt.Sprintf("interface{} (nil)")
		}
		return displayValue(v.Elem())
	default:
		return "unknown kind: " + v.Type().Kind().String()
	}
}

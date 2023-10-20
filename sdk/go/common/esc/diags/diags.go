// Copyright 2023, Pulumi Corporation.
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

package diags

import (
	"fmt"
	"strings"
)

// A formatter for when a field or property is used that does not exist.
type NonExistentFieldFormatter struct {
	ParentLabel         string
	Fields              []string
	MaxElements         int
	FieldsAreProperties bool
}

func (e NonExistentFieldFormatter) fieldsName() string {
	if e.FieldsAreProperties {
		return "properties"
	}
	return "fields"
}

func (e NonExistentFieldFormatter) messageHeader(fieldLabel string) string {
	return fmt.Sprintf("%s does not exist on %s", fieldLabel, e.ParentLabel)
}

func (e NonExistentFieldFormatter) messageBody(field string) string {
	existing := sortByEditDistance(e.Fields, field)
	if len(existing) == 0 {
		return fmt.Sprintf("%s has no %s", e.ParentLabel, e.fieldsName())
	}
	list := strings.Join(existing, ", ")
	if len(existing) > e.MaxElements && e.MaxElements != 0 {
		extraLength := len(existing) - e.MaxElements
		pluralOther := "others"
		if extraLength == 1 {
			pluralOther = "other"
		}
		list = fmt.Sprintf("%s and %d %s", strings.Join(existing[:e.MaxElements], ", "), extraLength, pluralOther)
	}
	return fmt.Sprintf("Existing %s are: %s", e.fieldsName(), list)
}

// Get a single line message.
func (e NonExistentFieldFormatter) Message(field, fieldLabel string) string {
	return fmt.Sprintf("%s. %s", e.messageHeader(fieldLabel), e.messageBody(field))
}

// A message broken up into a top level and detail line
func (e NonExistentFieldFormatter) MessageWithDetail(field, fieldLabel string) (string, string) {
	return e.messageHeader(fieldLabel), e.messageBody(field)
}

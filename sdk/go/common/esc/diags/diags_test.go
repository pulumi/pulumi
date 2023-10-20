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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNonExistentFieldFormatterFieldsName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		formatter NonExistentFieldFormatter
		want      string
	}{
		{NonExistentFieldFormatter{FieldsAreProperties: true}, "properties"},
		{NonExistentFieldFormatter{FieldsAreProperties: false}, "fields"},
	}
	for _, tt := range tests {
		assert.Equalf(t, tt.want, tt.formatter.fieldsName(), "fieldsName()")
	}
}

func TestNonExistentFieldFormatterMessageHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		formatter  NonExistentFieldFormatter
		fieldLabel string
		want       string
	}{
		{NonExistentFieldFormatter{ParentLabel: "parent"}, "label", "label does not exist on parent"},
	}
	for _, tt := range tests {
		assert.Equalf(t, tt.want, tt.formatter.messageHeader(tt.fieldLabel), "messageHeader(%s)", tt.fieldLabel)
	}
}

func TestNonExistentFieldFormatterMessageBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		formatter NonExistentFieldFormatter
		field     string
		want      string
	}{
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{}}, "field", "parent has no fields"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"a", "b"}}, "field", "Existing fields are: a, b"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field3", "field2", "field1"}}, "field", "Existing fields are: field1, field2, field3"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 4}, "field", "Existing fields are: field1, field2, field3"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 3}, "field", "Existing fields are: field1, field2, field3"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 2}, "field", "Existing fields are: field1, field2 and 1 other"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 1}, "field", "Existing fields are: field1 and 2 others"},
	}
	for _, tt := range tests {
		assert.Equalf(t, tt.want, tt.formatter.messageBody(tt.field), "messageBody(%s)", tt.field)
	}
}

func TestNonExistentFieldFormatterMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		formatter  NonExistentFieldFormatter
		field      string
		fieldLabel string
		want       string
	}{
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{}}, "field", "label", "label does not exist on parent. parent has no fields"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"a", "b"}}, "field", "label", "label does not exist on parent. Existing fields are: a, b"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field3", "field2", "field1"}}, "field", "label", "label does not exist on parent. Existing fields are: field1, field2, field3"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 2}, "field", "label", "label does not exist on parent. Existing fields are: field1, field2 and 1 other"},
	}
	for _, tt := range tests {
		assert.Equalf(t, tt.want, tt.formatter.Message(tt.field, tt.fieldLabel), "Message(%s, %s)", tt.field, tt.fieldLabel)
	}
}

func TestNonExistentFieldFormatterMessageWithDetail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		formatter  NonExistentFieldFormatter
		field      string
		fieldLabel string
		want1      string
		want2      string
	}{
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{}}, "field", "label", "label does not exist on parent", "parent has no fields"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"a", "b"}}, "field", "label", "label does not exist on parent", "Existing fields are: a, b"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field3", "field2", "field1"}}, "field", "label", "label does not exist on parent", "Existing fields are: field1, field2, field3"},
		{NonExistentFieldFormatter{ParentLabel: "parent", Fields: []string{"field1", "field2", "field3"}, MaxElements: 2}, "field", "label", "label does not exist on parent", "Existing fields are: field1, field2 and 1 other"},
	}
	for _, tt := range tests {
		got1, got2 := tt.formatter.MessageWithDetail(tt.field, tt.fieldLabel)
		assert.Equalf(t, tt.want1, got1, "MessageWithDetail(%v, %v)", tt.field, tt.fieldLabel)
		assert.Equalf(t, tt.want2, got2, "MessageWithDetail(%v, %v)", tt.field, tt.fieldLabel)
	}
}

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

package cmdutil

import (
	"regexp"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
)

func TestMeasureText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text     string
		expected int
	}{
		{
			text:     "",
			expected: 0,
		},
		{
			text:     "a",
			expected: 1,
		},
		{
			text:     "├",
			expected: 1,
		},
		{
			text:     "├─  ",
			expected: 4,
		},
		{
			text:     "\x1b[4m\x1b[38;5;12mType\x1b[0m",
			expected: 4,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.text, func(t *testing.T) {
			t.Parallel()

			count := MeasureText(c.text)
			assert.Equal(t, c.expected, count)
		})
	}
}

func TestTablePrinting(t *testing.T) {
	t.Parallel()

	rows := []TableRow{
		{Columns: []string{"A", "B", "C"}},
		{Columns: []string{"Some A", "B", "Some C"}},
	}

	table := &Table{
		Headers: []string{"ColumnA", "Long column B", "C"},
		Rows:    rows,
		Prefix:  "  ",
	}

	expected := "" +
		"  ColumnA  Long column B  C\n" +
		"  A        B              C\n" +
		"  Some A   B              Some C\n"

	assert.Equal(t, expected, table.ToStringWithGap("  "))
}

func TestColorTablePrinting(t *testing.T) {
	t.Parallel()

	greenText := func(msg string) string {
		return colors.Always.Colorize(colors.Green + msg + colors.Reset)
	}

	rows := []TableRow{
		{Columns: []string{greenText("+"), "pulumi:pulumi:Stack", "aws-cs-webserver-test", greenText("create")}},
		{Columns: []string{greenText("+"), "├─ aws:ec2/instance:Instance", "web-server-www", greenText("create")}},
		{Columns: []string{greenText("+"), "├─ aws:ec2/securityGroup:SecurityGroup", "web-secgrp", greenText("create")}},
		{Columns: []string{greenText("+"), "└─ pulumi:providers:aws", "default_4_25_0", greenText("create")}},
	}

	columnHeader := func(msg string) string {
		return colors.Always.Colorize(colors.Underline + colors.BrightBlue + msg + colors.Reset)
	}

	table := &Table{
		Headers: []string{"", columnHeader("Type"), columnHeader("Name"), columnHeader("Plan")},
		Rows:    rows,
		Prefix:  "  ",
	}

	expected := "" +
		"     Type                                    Name                   Plan\n" +
		"  +  pulumi:pulumi:Stack                     aws-cs-webserver-test  create\n" +
		"  +  ├─ aws:ec2/instance:Instance            web-server-www         create\n" +
		"  +  ├─ aws:ec2/securityGroup:SecurityGroup  web-secgrp             create\n" +
		"  +  └─ pulumi:providers:aws                 default_4_25_0         create\n"

	colorTable := table.ToStringWithGap("  ")
	// 7-bit C1 ANSI sequences
	ansiEscape := regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)
	cleanTable := ansiEscape.ReplaceAllString(colorTable, "")

	assert.Equal(t, expected, cleanTable)
}

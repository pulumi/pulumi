// Copyright 2024, Pulumi Corporation.
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

package style

import (
	"github.com/charmbracelet/glamour/ansi"
)

// Dark is the default dark style.
var Dark = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
			Color:       new("252"),
		},
		Margin: new(uint(2)),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Indent:         new(uint(1)),
		IndentToken:    new("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       new("39"),
			Bold:        new(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           new("228"),
			BackgroundColor: new("63"),
			Bold:            new(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Color:  new("35"),
			Bold:   new(false),
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: new(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: new(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: new(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  new("240"),
		Format: "\n--------\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
	},
	Task: ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{},
		Ticked:         "[✓] ",
		Unticked:       "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     new("30"),
		Underline: new(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: new("35"),
		Bold:  new(true),
	},
	Image: ansi.StylePrimitive{
		Color:     new("212"),
		Underline: new(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  new("243"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           new("203"),
			BackgroundColor: new("236"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: new("244"),
			},
			Margin: new(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: new("#C4C4C4"),
			},
			Error: ansi.StylePrimitive{
				Color:           new("#F1F1F1"),
				BackgroundColor: new("#F05B5B"),
			},
			Comment: ansi.StylePrimitive{
				Color: new("#676767"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: new("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: new("#00AAFF"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: new("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: new("#FF5F87"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: new("#6E6ED8"),
			},
			Operator: ansi.StylePrimitive{
				Color: new("#EF8080"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: new("#E8E8A8"),
			},
			Name: ansi.StylePrimitive{
				Color: new("#C4C4C4"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: new("#FF8EC7"),
			},
			NameTag: ansi.StylePrimitive{
				Color: new("#B083EA"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: new("#7A7AE6"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     new("#F1F1F1"),
				Underline: new(true),
				Bold:      new(true),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: new("#FFFF87"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: new("#00D787"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: new("#6EEFC0"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: new("#C69669"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: new("#AFFFD7"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: new("#FD5B5B"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: new(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: new("#00D787"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: new(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: new("#777777"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: new("#373737"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		CenterSeparator: new("┼"),
		ColumnSeparator: new("│"),
		RowSeparator:    new("─"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}

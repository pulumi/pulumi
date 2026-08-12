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
			Color:       ptr("252"),
		},
		Margin: ptr(uint(2)),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Indent:         ptr(uint(1)),
		IndentToken:    ptr("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       ptr("39"),
			Bold:        ptr(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           ptr("228"),
			BackgroundColor: ptr("63"),
			Bold:            ptr(true),
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
			Color:  ptr("35"),
			Bold:   ptr(false),
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: ptr(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: ptr(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: ptr(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  ptr("240"),
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
		Color:     ptr("30"),
		Underline: ptr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: ptr("35"),
		Bold:  ptr(true),
	},
	Image: ansi.StylePrimitive{
		Color:     ptr("212"),
		Underline: ptr(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  ptr("243"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           ptr("203"),
			BackgroundColor: ptr("236"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: ptr("244"),
			},
			Margin: ptr(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: ptr("#C4C4C4"),
			},
			Error: ansi.StylePrimitive{
				Color:           ptr("#F1F1F1"),
				BackgroundColor: ptr("#F05B5B"),
			},
			Comment: ansi.StylePrimitive{
				Color: ptr("#676767"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: ptr("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: ptr("#00AAFF"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: ptr("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: ptr("#FF5F87"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: ptr("#6E6ED8"),
			},
			Operator: ansi.StylePrimitive{
				Color: ptr("#EF8080"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: ptr("#E8E8A8"),
			},
			Name: ansi.StylePrimitive{
				Color: ptr("#C4C4C4"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: ptr("#FF8EC7"),
			},
			NameTag: ansi.StylePrimitive{
				Color: ptr("#B083EA"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: ptr("#7A7AE6"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     ptr("#F1F1F1"),
				Underline: ptr(true),
				Bold:      ptr(true),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: ptr("#FFFF87"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: ptr("#00D787"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: ptr("#6EEFC0"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: ptr("#C69669"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: ptr("#AFFFD7"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: ptr("#FD5B5B"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: ptr(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: ptr("#00D787"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: ptr(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: ptr("#777777"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: ptr("#373737"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		CenterSeparator: ptr("┼"),
		ColumnSeparator: ptr("│"),
		RowSeparator:    ptr("─"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}

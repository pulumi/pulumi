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

// Light is the default light style.
var Light = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
			Color:       ptr("234"),
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
			Color:       ptr("27"),
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
		Color:  ptr("249"),
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
		Color:     ptr("36"),
		Underline: ptr(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: ptr("29"),
		Bold:  ptr(true),
	},
	Image: ansi.StylePrimitive{
		Color:     ptr("205"),
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
			BackgroundColor: ptr("254"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: ptr("242"),
			},
			Margin: ptr(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: ptr("#2A2A2A"),
			},
			Error: ansi.StylePrimitive{
				Color:           ptr("#F1F1F1"),
				BackgroundColor: ptr("#FF5555"),
			},
			Comment: ansi.StylePrimitive{
				Color: ptr("#8D8D8D"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: ptr("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: ptr("#279EFC"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: ptr("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: ptr("#FB406F"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: ptr("#7049C2"),
			},
			Operator: ansi.StylePrimitive{
				Color: ptr("#FF2626"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: ptr("#FA7878"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: ptr("#0A1BB1"),
			},
			NameTag: ansi.StylePrimitive{
				Color: ptr("#581290"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: ptr("#8362CB"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     ptr("#212121"),
				Underline: ptr(true),
				Bold:      ptr(true),
			},
			NameConstant: ansi.StylePrimitive{
				Color: ptr("#581290"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: ptr("#A3A322"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: ptr("#019F57"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: ptr("#22CCAE"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: ptr("#7E5B38"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: ptr("#00AEAE"),
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

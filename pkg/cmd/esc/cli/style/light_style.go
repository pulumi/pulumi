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
			Color:       new("234"),
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
			Color:       new("27"),
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
		Color:  new("249"),
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
		Color:     new("36"),
		Underline: new(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: new("29"),
		Bold:  new(true),
	},
	Image: ansi.StylePrimitive{
		Color:     new("205"),
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
			BackgroundColor: new("254"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: new("242"),
			},
			Margin: new(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: new("#2A2A2A"),
			},
			Error: ansi.StylePrimitive{
				Color:           new("#F1F1F1"),
				BackgroundColor: new("#FF5555"),
			},
			Comment: ansi.StylePrimitive{
				Color: new("#8D8D8D"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: new("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: new("#279EFC"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: new("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: new("#FB406F"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: new("#7049C2"),
			},
			Operator: ansi.StylePrimitive{
				Color: new("#FF2626"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: new("#FA7878"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: new("#0A1BB1"),
			},
			NameTag: ansi.StylePrimitive{
				Color: new("#581290"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: new("#8362CB"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     new("#212121"),
				Underline: new(true),
				Bold:      new(true),
			},
			NameConstant: ansi.StylePrimitive{
				Color: new("#581290"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: new("#A3A322"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: new("#019F57"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: new("#22CCAE"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: new("#7E5B38"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: new("#00AEAE"),
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

// Copyright 2024, Pulumi Corporation.

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
			Color:       some("234"),
		},
		Margin: some(uint(2)),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{},
		Indent:         some(uint(1)),
		IndentToken:    some("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       some("27"),
			Bold:        some(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           some("228"),
			BackgroundColor: some("63"),
			Bold:            some(true),
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
			Bold:   some(false),
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: some(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: some(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: some(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  some("249"),
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
		Color:     some("36"),
		Underline: some(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: some("29"),
		Bold:  some(true),
	},
	Image: ansi.StylePrimitive{
		Color:     some("205"),
		Underline: some(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  some("243"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           some("203"),
			BackgroundColor: some("254"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: some("242"),
			},
			Margin: some(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: some("#2A2A2A"),
			},
			Error: ansi.StylePrimitive{
				Color:           some("#F1F1F1"),
				BackgroundColor: some("#FF5555"),
			},
			Comment: ansi.StylePrimitive{
				Color: some("#8D8D8D"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: some("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: some("#279EFC"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: some("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: some("#FB406F"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: some("#7049C2"),
			},
			Operator: ansi.StylePrimitive{
				Color: some("#FF2626"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: some("#FA7878"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: some("#0A1BB1"),
			},
			NameTag: ansi.StylePrimitive{
				Color: some("#581290"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: some("#8362CB"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     some("#212121"),
				Underline: some(true),
				Bold:      some(true),
			},
			NameConstant: ansi.StylePrimitive{
				Color: some("#581290"),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: some("#A3A322"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: some("#019F57"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: some("#22CCAE"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: some("#7E5B38"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: some("#00AEAE"),
			},
			GenericDeleted: ansi.StylePrimitive{
				Color: some("#FD5B5B"),
			},
			GenericEmph: ansi.StylePrimitive{
				Italic: some(true),
			},
			GenericInserted: ansi.StylePrimitive{
				Color: some("#00D787"),
			},
			GenericStrong: ansi.StylePrimitive{
				Bold: some(true),
			},
			GenericSubheading: ansi.StylePrimitive{
				Color: some("#777777"),
			},
			Background: ansi.StylePrimitive{
				BackgroundColor: some("#373737"),
			},
		},
	},
	Table: ansi.StyleTable{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		CenterSeparator: some("┼"),
		ColumnSeparator: some("│"),
		RowSeparator:    some("─"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}

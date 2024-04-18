// Copyright 2024, Pulumi Corporation.

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
			Color:       some("252"),
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
			Color:       some("39"),
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
			Color:  some("35"),
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
		Color:  some("240"),
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
		Color:     some("30"),
		Underline: some(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: some("35"),
		Bold:  some(true),
	},
	Image: ansi.StylePrimitive{
		Color:     some("212"),
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
			BackgroundColor: some("236"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: some("244"),
			},
			Margin: some(uint(2)),
		},
		Chroma: &ansi.Chroma{
			Text: ansi.StylePrimitive{
				Color: some("#C4C4C4"),
			},
			Error: ansi.StylePrimitive{
				Color:           some("#F1F1F1"),
				BackgroundColor: some("#F05B5B"),
			},
			Comment: ansi.StylePrimitive{
				Color: some("#676767"),
			},
			CommentPreproc: ansi.StylePrimitive{
				Color: some("#FF875F"),
			},
			Keyword: ansi.StylePrimitive{
				Color: some("#00AAFF"),
			},
			KeywordReserved: ansi.StylePrimitive{
				Color: some("#FF5FD2"),
			},
			KeywordNamespace: ansi.StylePrimitive{
				Color: some("#FF5F87"),
			},
			KeywordType: ansi.StylePrimitive{
				Color: some("#6E6ED8"),
			},
			Operator: ansi.StylePrimitive{
				Color: some("#EF8080"),
			},
			Punctuation: ansi.StylePrimitive{
				Color: some("#E8E8A8"),
			},
			Name: ansi.StylePrimitive{
				Color: some("#C4C4C4"),
			},
			NameBuiltin: ansi.StylePrimitive{
				Color: some("#FF8EC7"),
			},
			NameTag: ansi.StylePrimitive{
				Color: some("#B083EA"),
			},
			NameAttribute: ansi.StylePrimitive{
				Color: some("#7A7AE6"),
			},
			NameClass: ansi.StylePrimitive{
				Color:     some("#F1F1F1"),
				Underline: some(true),
				Bold:      some(true),
			},
			NameDecorator: ansi.StylePrimitive{
				Color: some("#FFFF87"),
			},
			NameFunction: ansi.StylePrimitive{
				Color: some("#00D787"),
			},
			LiteralNumber: ansi.StylePrimitive{
				Color: some("#6EEFC0"),
			},
			LiteralString: ansi.StylePrimitive{
				Color: some("#C69669"),
			},
			LiteralStringEscape: ansi.StylePrimitive{
				Color: some("#AFFFD7"),
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

package parsing

import (
	"fmt"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Value struct {
	Object  *Object `  @@`
	Array   *Array  `| @@`
	String_ *string `| @String`
	Number  *Number `| @@`
	Null    *bool   `| "null"`
	Bool    *bool   `| (@"true" | "false")`
}

func (v *Value) String() string {
	switch {
	case v.Object != nil:
		return v.Object.String()
	case v.Array != nil:
		return v.Array.String()
	case v.String_ != nil:
		return *v.String_
	case v.Number != nil:
		return v.Number.String()
	case v.Null != nil:
		return "null"
	case v.Bool != nil:
		if *v.Bool {
			return "true"
		}
		return "false"
	default:
		panic("Value not initialized")
	}
}

type Object struct {
	Members []*Member `"{" ( @@ ( "," @@ )* )? "}"`
}

func (o *Object) String() string {
	if len(o.Members) == 0 {
		return "{ }"
	}
	s := "{ " + o.Members[0].String()
	for i := 1; i < len(o.Members); i++ {
		s = s + ", " + o.Members[i].String()
	}
	return s + " }"
}

type Member struct {
	Key   *string `@String ":"`
	Value *Value  `@@`
}

func (m Member) String() string {
	return *m.Key + ": " + (*m.Value).String()
}

type Array struct {
	Entries []*Value `"[" ( @@ ( "," @@ )* )? "]"`
}

func (a Array) String() string {
	if len(a.Entries) == 0 {
		return "[ ]"
	}
	s := "[ " + a.Entries[0].String()
	for i := 1; i < len(a.Entries); i++ {
		s = s + ", " + a.Entries[i].String()
	}
	return s + " ]"
}

type Number struct {
	Integer  *Integer  `@@`
	Fraction *Fraction `@@`
	Exponent *Exponent `@@`
}

type Integer struct {
	Minus *char   `@"-"`
	digit *string `@Digit`
	rest  *string `@OneNine`
}

func (n Number) String() string {
	return *n.Integer
}

func ParseJSON(s string) {
	jsonLexer := lexer.Must(lexer.NewSimple([]lexer.Rule{
		{"Digit", `[0-9]`, nil},
		{"OneNine", `[1-9]`, nil},
		{"whitespace", `\s+`, nil},
		{"String", `"[^"]*"`, nil},
		{"Punct", `[\{\}\[\],:]`, nil},
		{"Null", "null", nil},
		{"Bool", "true|false", nil},
	}))

	jsonParser := participle.MustBuild(&Value{}, participle.Lexer(jsonLexer), participle.Unquote("String"))

	json := &Value{}
	err := jsonParser.ParseString("schema", s, json)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Printf("JSON parsed to %v\n", json)
	}
}

package syntax

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var tokenStrings = map[hclsyntax.TokenType]string{
	hclsyntax.TokenOBrace: "{",
	hclsyntax.TokenCBrace: "}",
	hclsyntax.TokenOBrack: "[",
	hclsyntax.TokenCBrack: "]",
	hclsyntax.TokenOParen: "(",
	hclsyntax.TokenCParen: ")",
	hclsyntax.TokenOQuote: `"`,
	hclsyntax.TokenCQuote: `"`,

	hclsyntax.TokenStar:    "*",
	hclsyntax.TokenSlash:   "/",
	hclsyntax.TokenPlus:    "+",
	hclsyntax.TokenMinus:   "-",
	hclsyntax.TokenPercent: "%",

	hclsyntax.TokenEqual:         "=",
	hclsyntax.TokenEqualOp:       "==",
	hclsyntax.TokenNotEqual:      "!=",
	hclsyntax.TokenLessThan:      "<",
	hclsyntax.TokenLessThanEq:    "<=",
	hclsyntax.TokenGreaterThan:   ">",
	hclsyntax.TokenGreaterThanEq: ">=",

	hclsyntax.TokenAnd:  "&&",
	hclsyntax.TokenOr:   "||",
	hclsyntax.TokenBang: "!",

	hclsyntax.TokenDot:   ".",
	hclsyntax.TokenComma: ",",

	hclsyntax.TokenEllipsis: "...",
	hclsyntax.TokenFatArrow: "=>",

	hclsyntax.TokenQuestion: "?",
	hclsyntax.TokenColon:    ":",

	hclsyntax.TokenTemplateInterp:  "${",
	hclsyntax.TokenTemplateControl: "%{",
	hclsyntax.TokenTemplateSeqEnd:  "}",

	hclsyntax.TokenNewline: "\n",
}

// Trivia represents bytes in a source file that are not syntactically meaningful. This includes whitespace and
// comments.
type Trivia interface {
	// Range returns the range of the trivia in the source file.
	Range() hcl.Range
	// Bytes returns the raw bytes that comprise the trivia.
	Bytes() []byte

	isTrivia()
}

// TriviaList is a list of trivia.
type TriviaList []Trivia

func (trivia TriviaList) CollapseWhitespace() TriviaList {
	result := make(TriviaList, 0, len(trivia))
	for _, t := range trivia {
		if ws, ok := t.(Whitespace); ok {
			if len(result) != 0 {
				ws.bytes = []byte{' '}
				result = append(result, ws)
			}
		} else {
			result = append(result, t)
		}
	}
	return result
}

func (trivia TriviaList) Format(f fmt.State, c rune) {
	for _, trivia := range trivia {
		_, err := f.Write(trivia.Bytes())
		if err != nil {
			panic(err)
		}
	}
}

// Comment is a piece of trivia that represents a line or block comment in a source file.
type Comment struct {
	// Lines contains the lines of the comment without leading comment characters or trailing newlines.
	Lines []string

	rng   hcl.Range
	bytes []byte
}

// Range returns the range of the comment in the source file.
func (c Comment) Range() hcl.Range {
	return c.rng
}

// Bytes returns the raw bytes that comprise the comment.
func (c Comment) Bytes() []byte {
	return c.bytes
}

func (Comment) isTrivia() {}

// Whitespace is a piece of trivia that represents a sequence of whitespace characters in a source file.
type Whitespace struct {
	rng   hcl.Range
	bytes []byte
}

// NewWhitespace returns a new piece of whitespace trivia with the given contents.
func NewWhitespace(bytes ...byte) Whitespace {
	return Whitespace{bytes: bytes}
}

// Range returns the range of the whitespace in the source file.
func (w Whitespace) Range() hcl.Range {
	return w.rng
}

// Bytes returns the raw bytes that comprise the whitespace.
func (w Whitespace) Bytes() []byte {
	return w.bytes
}

func (Whitespace) isTrivia() {}

// TemplateDelimiter is a piece of trivia that represents a token that demarcates an interpolation or control sequence
// inside of a template.
type TemplateDelimiter struct {
	// Type is the type of the delimiter (e.g. hclsyntax.TokenTemplateInterp)
	Type hclsyntax.TokenType

	rng   hcl.Range
	bytes []byte
}

// Range returns the range of the delimiter in the source file.
func (t TemplateDelimiter) Range() hcl.Range {
	return t.rng
}

// Bytes returns the raw bytes that comprise the delimiter.
func (t TemplateDelimiter) Bytes() []byte {
	return t.bytes
}

func (TemplateDelimiter) isTrivia() {}

// Token represents an HCL2 syntax token with attached leading trivia.
type Token struct {
	Raw            hclsyntax.Token
	LeadingTrivia  TriviaList
	TrailingTrivia TriviaList
}

func (t Token) Format(f fmt.State, c rune) {
	if t.LeadingTrivia != nil {
		t.LeadingTrivia.Format(f, c)
	} else if f.Flag(' ') {
		if _, err := f.Write([]byte{' '}); err != nil {
			panic(err)
		}
	}
	bytes := t.Raw.Bytes
	if str, ok := tokenStrings[t.Raw.Type]; ok {
		bytes = []byte(str)
	}
	if _, err := f.Write(bytes); err != nil {
		panic(err)
	}
	t.TrailingTrivia.Format(f, c)
}

func (t Token) AllTrivia() TriviaList {
	result := make(TriviaList, len(t.LeadingTrivia)+len(t.TrailingTrivia))
	for i, trivia := range t.LeadingTrivia {
		result[i] = trivia
	}
	for i, trivia := range t.TrailingTrivia {
		result[len(t.LeadingTrivia)+i] = trivia
	}
	return result
}

// Range returns the total range covered by this token and any leading trivia.
func (t Token) Range() hcl.Range {
	start := t.Raw.Range.Start
	if len(t.LeadingTrivia) > 0 {
		start = t.LeadingTrivia[0].Range().Start
	}
	end := t.Raw.Range.End
	if len(t.TrailingTrivia) > 0 {
		end = t.TrailingTrivia[len(t.TrailingTrivia)-1].Range().End
	}
	return hcl.Range{Filename: t.Raw.Range.Filename, Start: start, End: end}
}

func (t Token) Or(typ hclsyntax.TokenType, s ...string) Token {
	t.Raw.Type = typ
	if len(s) > 0 {
		t.Raw.Bytes = []byte(s[0])
	}
	return t
}

func OperationTokenType(operation *hclsyntax.Operation) hclsyntax.TokenType {
	switch operation {
	case hclsyntax.OpAdd:
		return hclsyntax.TokenPlus
	case hclsyntax.OpDivide:
		return hclsyntax.TokenSlash
	case hclsyntax.OpEqual:
		return hclsyntax.TokenEqualOp
	case hclsyntax.OpGreaterThan:
		return hclsyntax.TokenGreaterThan
	case hclsyntax.OpGreaterThanOrEqual:
		return hclsyntax.TokenGreaterThanEq
	case hclsyntax.OpLessThan:
		return hclsyntax.TokenLessThan
	case hclsyntax.OpLessThanOrEqual:
		return hclsyntax.TokenLessThanEq
	case hclsyntax.OpLogicalAnd:
		return hclsyntax.TokenAnd
	case hclsyntax.OpLogicalNot:
		return hclsyntax.TokenBang
	case hclsyntax.OpLogicalOr:
		return hclsyntax.TokenOr
	case hclsyntax.OpModulo:
		return hclsyntax.TokenPercent
	case hclsyntax.OpMultiply:
		return hclsyntax.TokenStar
	case hclsyntax.OpNegate:
		return hclsyntax.TokenMinus
	case hclsyntax.OpNotEqual:
		return hclsyntax.TokenNotEqual
	case hclsyntax.OpSubtract:
		return hclsyntax.TokenMinus
	}
	return hclsyntax.TokenInvalid
}

func newRawToken(typ hclsyntax.TokenType, text ...string) hclsyntax.Token {
	bytes := []byte(tokenStrings[typ])
	if len(text) != 0 {
		bytes = []byte(text[0])
	}
	return hclsyntax.Token{
		Type:  typ,
		Bytes: bytes,
	}
}

// NodeTokens is a closed interface that is used to represent arbitrary *Tokens types in this package.
type NodeTokens interface {
	isNodeTokens()
}

// AttributeTokens records the tokens associated with an *hclsyntax.Attribute.
type AttributeTokens struct {
	Name   Token
	Equals Token
}

func NewAttributeTokens(name string) *AttributeTokens {
	return &AttributeTokens{
		Name: Token{
			Raw: newRawToken(hclsyntax.TokenIdent, name),
		},
		Equals: Token{
			Raw:           newRawToken(hclsyntax.TokenEqual),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
	}
}

func (t *AttributeTokens) GetName() Token {
	if t == nil {
		return Token{}
	}
	return t.Name
}

func (t *AttributeTokens) GetEquals() Token {
	if t == nil {
		return Token{}
	}
	return t.Equals
}

func (*AttributeTokens) isNodeTokens() {}

// BinaryOpTokens records the tokens associated with an *hclsyntax.BinaryOpExpr.
type BinaryOpTokens struct {
	Operator Token
}

func NewBinaryOpTokens(operation *hclsyntax.Operation) *BinaryOpTokens {
	operatorType := OperationTokenType(operation)
	return &BinaryOpTokens{
		Operator: Token{
			Raw:           newRawToken(operatorType),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
	}
}

func (t *BinaryOpTokens) GetOperator() Token {
	if t == nil {
		return Token{}
	}
	return t.Operator
}

func (*BinaryOpTokens) isNodeTokens() {}

// BlockTokens records the tokens associated with an *hclsyntax.Block.
type BlockTokens struct {
	Type       Token
	Labels     []Token
	OpenBrace  Token
	CloseBrace Token
}

func NewBlockTokens(typ string, labels ...string) *BlockTokens {
	labelTokens := make([]Token, len(labels))
	for i, l := range labels {
		var raw hclsyntax.Token
		if hclsyntax.ValidIdentifier(l) {
			raw = newRawToken(hclsyntax.TokenIdent, l)
		} else {
			raw = newRawToken(hclsyntax.TokenQuotedLit, fmt.Sprintf("%q", l))
		}
		labelTokens[i] = Token{
			Raw:           raw,
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
	}
	return &BlockTokens{
		Type: Token{
			Raw: newRawToken(hclsyntax.TokenIdent, typ),
		},
		Labels: labelTokens,
		OpenBrace: Token{
			Raw:           newRawToken(hclsyntax.TokenOBrace),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		CloseBrace: Token{
			Raw:           newRawToken(hclsyntax.TokenCBrace),
			LeadingTrivia: TriviaList{NewWhitespace('\n')},
		},
	}
}

func (t *BlockTokens) GetType() Token {
	if t == nil {
		return Token{}
	}
	return t.Type
}

func (t *BlockTokens) GetLabels() []Token {
	if t == nil {
		return nil
	}
	return t.Labels
}

func (t *BlockTokens) GetOpenBrace() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenBrace
}

func (t *BlockTokens) GetCloseBrace() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseBrace
}

func (*BlockTokens) isNodeTokens() {}

// BodyTokens records the tokens associated with an *hclsyntax.Body.
type BodyTokens struct {
	EndOfFile *Token
}

func (t *BodyTokens) GetEndOfFile() *Token {
	if t == nil {
		return nil
	}
	return t.EndOfFile
}

func (*BodyTokens) isNodeTokens() {}

// ConditionalTokens records the tokens associated with an *hclsyntax.ConditionalExpr of the form "a ? t : f".
type ConditionalTokens struct {
	QuestionMark Token
	Colon        Token
}

func NewConditionalTokens() *ConditionalTokens {
	return &ConditionalTokens{
		QuestionMark: Token{
			Raw:           newRawToken(hclsyntax.TokenQuestion),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		Colon: Token{
			Raw:           newRawToken(hclsyntax.TokenColon),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
	}
}

func (t *ConditionalTokens) GetQuestionMark() Token {
	if t == nil {
		return Token{}
	}
	return t.QuestionMark
}

func (t *ConditionalTokens) GetColon() Token {
	if t == nil {
		return Token{}
	}
	return t.Colon
}

func (*ConditionalTokens) isNodeTokens() {}

// TemplateConditionalTokens records the tokens associated with an *hclsyntax.ConditionalExpr inside a template
// expression.
type TemplateConditionalTokens struct {
	OpenIf     Token
	If         Token
	CloseIf    Token
	OpenElse   *Token
	Else       *Token
	CloseElse  *Token
	OpenEndif  Token
	Endif      Token
	CloseEndif Token
}

func NewTemplateConditionalTokens(hasElse bool) *TemplateConditionalTokens {
	var openElseT, elseT, closeElseT *Token
	if hasElse {
		openElseT = &Token{Raw: newRawToken(hclsyntax.TokenTemplateControl)}
		elseT = &Token{Raw: newRawToken(hclsyntax.TokenIdent, "else")}
		closeElseT = &Token{Raw: newRawToken(hclsyntax.TokenTemplateSeqEnd)}
	}
	return &TemplateConditionalTokens{
		OpenIf:     Token{Raw: newRawToken(hclsyntax.TokenTemplateControl)},
		If:         Token{Raw: newRawToken(hclsyntax.TokenIdent, "if")},
		CloseIf:    Token{Raw: newRawToken(hclsyntax.TokenTemplateSeqEnd)},
		OpenElse:   openElseT,
		Else:       elseT,
		CloseElse:  closeElseT,
		OpenEndif:  Token{Raw: newRawToken(hclsyntax.TokenTemplateControl)},
		Endif:      Token{Raw: newRawToken(hclsyntax.TokenIdent, "endif")},
		CloseEndif: Token{Raw: newRawToken(hclsyntax.TokenTemplateSeqEnd)},
	}
}

func (*TemplateConditionalTokens) isNodeTokens() {}

// ForTokens records the tokens associated with an *hclsyntax.ForExpr.
type ForTokens struct {
	Open  Token
	For   Token
	Key   *Token
	Comma *Token
	Value Token
	In    Token
	Colon Token
	Arrow *Token
	Group *Token
	If    *Token
	Close Token
}

func NewForTokens(keyVariable, valueVariable string, mapFor, group, conditional bool) *ForTokens {
	var keyT, commaT, arrowT, groupT, ifT *Token
	if keyVariable != "" {
		keyT = &Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, keyVariable),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
		commaT = &Token{Raw: newRawToken(hclsyntax.TokenComma)}
	}
	if mapFor {
		arrowT = &Token{
			Raw:           newRawToken(hclsyntax.TokenFatArrow),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
	}
	if group {
		groupT = &Token{Raw: newRawToken(hclsyntax.TokenEllipsis)}
	}
	if conditional {
		ifT = &Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, "if"),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
	}

	return &ForTokens{
		Open:  Token{Raw: newRawToken(hclsyntax.TokenOBrack)},
		For:   Token{Raw: newRawToken(hclsyntax.TokenIdent, "for")},
		Key:   keyT,
		Comma: commaT,
		Value: Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, valueVariable),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		In: Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, "in"),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		Colon: Token{Raw: newRawToken(hclsyntax.TokenColon)},
		Arrow: arrowT,
		Group: groupT,
		If:    ifT,
		Close: Token{Raw: newRawToken(hclsyntax.TokenCBrack)},
	}
}

func (t *ForTokens) GetOpen() Token {
	if t == nil {
		return Token{}
	}
	return t.Open
}

func (t *ForTokens) GetFor() Token {
	if t == nil {
		return Token{}
	}
	return t.For
}

func (t *ForTokens) GetKey() *Token {
	if t == nil {
		return nil
	}
	return t.Key
}

func (t *ForTokens) GetComma() *Token {
	if t == nil {
		return nil
	}
	return t.Comma
}

func (t *ForTokens) GetValue() Token {
	if t == nil {
		return Token{}
	}
	return t.Value
}

func (t *ForTokens) GetIn() Token {
	if t == nil {
		return Token{}
	}
	return t.In
}

func (t *ForTokens) GetColon() Token {
	if t == nil {
		return Token{}
	}
	return t.Colon
}

func (t *ForTokens) GetArrow() *Token {
	if t == nil {
		return nil
	}
	return t.Arrow
}

func (t *ForTokens) GetGroup() *Token {
	if t == nil {
		return nil
	}
	return t.Group
}

func (t *ForTokens) GetIf() *Token {
	if t == nil {
		return nil
	}
	return t.If
}

func (t *ForTokens) GetClose() Token {
	if t == nil {
		return Token{}
	}
	return t.Close
}

func (*ForTokens) isNodeTokens() {}

// TemplateForTokens records the tokens associated with an *hclsyntax.ForExpr inside a template.
type TemplateForTokens struct {
	OpenFor     Token
	For         Token
	Key         *Token
	Comma       *Token
	Value       Token
	In          Token
	CloseFor    Token
	OpenEndfor  Token
	Endfor      Token
	CloseEndfor Token
}

func NewTemplateForTokens(keyVariable, valueVariable string) *TemplateForTokens {
	var keyT, commaT *Token
	if keyVariable != "" {
		keyT = &Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, keyVariable),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
		commaT = &Token{Raw: newRawToken(hclsyntax.TokenComma)}
	}

	return &TemplateForTokens{
		OpenFor: Token{Raw: newRawToken(hclsyntax.TokenTemplateControl)},
		For:     Token{Raw: newRawToken(hclsyntax.TokenIdent, "for")},
		Key:     keyT,
		Comma:   commaT,
		Value: Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, valueVariable),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		In: Token{
			Raw:           newRawToken(hclsyntax.TokenIdent, "in"),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		CloseFor:    Token{Raw: newRawToken(hclsyntax.TokenTemplateSeqEnd)},
		OpenEndfor:  Token{Raw: newRawToken(hclsyntax.TokenTemplateControl)},
		Endfor:      Token{Raw: newRawToken(hclsyntax.TokenIdent, "endfor")},
		CloseEndfor: Token{Raw: newRawToken(hclsyntax.TokenTemplateSeqEnd)},
	}
}

func (t *TemplateForTokens) GetFor() Token {
	if t == nil {
		return Token{}
	}
	return t.For
}

func (t *TemplateForTokens) GetKey() *Token {
	if t == nil {
		return nil
	}
	return t.Key
}

func (t *TemplateForTokens) GetComma() *Token {
	if t == nil {
		return nil
	}
	return t.Comma
}

func (t *TemplateForTokens) GetValue() Token {
	if t == nil {
		return Token{}
	}
	return t.Value
}

func (t *TemplateForTokens) GetIn() Token {
	if t == nil {
		return Token{}
	}
	return t.In
}

func (t *TemplateForTokens) GetEndfor() Token {
	if t == nil {
		return Token{}
	}
	return t.Endfor
}

func (*TemplateForTokens) isNodeTokens() {}

// FunctionCallTokens records the tokens associated with an *hclsyntax.FunctionCallExpr.
type FunctionCallTokens struct {
	Name       Token
	OpenParen  Token
	Commas     []Token
	CloseParen Token
}

func NewFunctionCallTokens(name string, argCount int) *FunctionCallTokens {
	commas := make([]Token, argCount-1)
	for i := 0; i < len(commas); i++ {
		commas[i] = Token{Raw: newRawToken(hclsyntax.TokenComma)}
	}
	return &FunctionCallTokens{
		Name:       Token{Raw: newRawToken(hclsyntax.TokenIdent, name)},
		OpenParen:  Token{Raw: newRawToken(hclsyntax.TokenOParen)},
		Commas:     commas,
		CloseParen: Token{Raw: newRawToken(hclsyntax.TokenCParen)},
	}
}

func (t *FunctionCallTokens) GetName() Token {
	if t == nil {
		return Token{}
	}
	return t.Name
}

func (t *FunctionCallTokens) GetOpenParen() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenParen
}

func (t *FunctionCallTokens) GetCommas() []Token {
	if t == nil {
		return nil
	}
	return t.Commas
}

func (t *FunctionCallTokens) GetCloseParen() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseParen
}

func (*FunctionCallTokens) isNodeTokens() {}

// IndexTokens records the tokens associated with an *hclsyntax.IndexExpr.
type IndexTokens struct {
	OpenBracket  Token
	CloseBracket Token
}

func NewIndexTokens() *IndexTokens {
	return &IndexTokens{
		OpenBracket:  Token{Raw: newRawToken(hclsyntax.TokenOBrack)},
		CloseBracket: Token{Raw: newRawToken(hclsyntax.TokenCBrack)},
	}
}

func (t *IndexTokens) GetOpenBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenBracket
}

func (t *IndexTokens) GetCloseBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseBracket
}

func (*IndexTokens) isNodeTokens() {}

// LiteralValueTokens records the tokens associated with an *hclsyntax.LiteralValueExpr.
type LiteralValueTokens struct {
	Value []Token
}

func NewLiteralValueTokens(tokens ...Token) *LiteralValueTokens {
	return &LiteralValueTokens{
		Value: tokens,
	}
}

func (t *LiteralValueTokens) GetValue() []Token {
	if t == nil {
		return nil
	}
	return t.Value
}

func (*LiteralValueTokens) isNodeTokens() {}

// ObjectConsItemTokens records the tokens associated with an hclsyntax.ObjectConsItem.
type ObjectConsItemTokens struct {
	Equals Token
	Comma  *Token
}

// ObjectConsTokens records the tokens associated with an *hclsyntax.ObjectConsExpr.
type ObjectConsTokens struct {
	OpenBrace  Token
	Items      []ObjectConsItemTokens
	CloseBrace Token
}

func NewObjectConsTokens(itemCount int) *ObjectConsTokens {
	items := make([]ObjectConsItemTokens, itemCount)
	for i := 0; i < len(items); i++ {
		var comma *Token
		if i < len(items)-1 {
			comma = &Token{Raw: newRawToken(hclsyntax.TokenComma)}
		}
		items[i] = ObjectConsItemTokens{
			Equals: Token{Raw: newRawToken(hclsyntax.TokenEqual)},
			Comma:  comma,
		}
	}
	return &ObjectConsTokens{
		OpenBrace:  Token{Raw: newRawToken(hclsyntax.TokenOBrace)},
		Items:      items,
		CloseBrace: Token{Raw: newRawToken(hclsyntax.TokenCBrace)},
	}
}

func (t *ObjectConsTokens) GetOpenBrace() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenBrace
}

func (t *ObjectConsTokens) GetItems() []ObjectConsItemTokens {
	if t == nil {
		return nil
	}
	return t.Items
}

func (t *ObjectConsTokens) GetCloseBrace() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseBrace
}

func (*ObjectConsTokens) isNodeTokens() {}

// TraverserTokens is a closed interface implemented by DotTraverserTokens and BracketTraverserTokens
type TraverserTokens interface {
	GetIndex() Token

	isTraverserTokens()
}

// DotTraverserTokens records the tokens associated with dotted traverser (i.e. '.' <attr>).
type DotTraverserTokens struct {
	Dot   Token
	Index Token
}

func NewDotTraverserTokens(index string) *DotTraverserTokens {
	indexType := hclsyntax.TokenIdent
	_, err := cty.ParseNumberVal(index)
	if err == nil {
		indexType = hclsyntax.TokenNumberLit
	}
	return &DotTraverserTokens{
		Dot:   Token{Raw: newRawToken(hclsyntax.TokenDot)},
		Index: Token{Raw: newRawToken(indexType, index)},
	}
}

func (t *DotTraverserTokens) GetDot() Token {
	if t == nil {
		return Token{}
	}
	return t.Dot
}

func (t *DotTraverserTokens) GetIndex() Token {
	if t == nil {
		return Token{}
	}
	return t.Index
}

func (*DotTraverserTokens) isTraverserTokens() {}

// BracketTraverserTokens records the tokens associated with a bracketed traverser (i.e. '[' <index> ']').
type BracketTraverserTokens struct {
	OpenBracket  Token
	Index        Token
	CloseBracket Token
}

func NewBracketTraverserTokens(index string) *BracketTraverserTokens {
	indexType := hclsyntax.TokenIdent
	_, err := cty.ParseNumberVal(index)
	if err == nil {
		indexType = hclsyntax.TokenNumberLit
	}
	return &BracketTraverserTokens{
		OpenBracket:  Token{Raw: newRawToken(hclsyntax.TokenOBrack)},
		Index:        Token{Raw: newRawToken(indexType, index)},
		CloseBracket: Token{Raw: newRawToken(hclsyntax.TokenCBrack)},
	}
}

func (t *BracketTraverserTokens) GetOpenBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenBracket
}

func (t *BracketTraverserTokens) GetIndex() Token {
	if t == nil {
		return Token{}
	}
	return t.Index
}

func (t *BracketTraverserTokens) GetCloseBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseBracket
}

func (*BracketTraverserTokens) isTraverserTokens() {}

// RelativeTraversalTokens records the tokens associated with an *hclsyntax.RelativeTraversalExpr.
type RelativeTraversalTokens struct {
	Traversal []TraverserTokens
}

func NewRelativeTraversalTokens(traversers ...TraverserTokens) *RelativeTraversalTokens {
	return &RelativeTraversalTokens{Traversal: traversers}
}

func (t *RelativeTraversalTokens) GetTraversal() []TraverserTokens {
	if t == nil {
		return nil
	}
	return t.Traversal
}

func (*RelativeTraversalTokens) isNodeTokens() {}

// ScopeTraversalTokens records the tokens associated with an *hclsyntax.ScopeTraversalExpr.
type ScopeTraversalTokens struct {
	Root      Token
	Traversal []TraverserTokens
}

func NewScopeTraversalTokens(root string, traversers ...TraverserTokens) *ScopeTraversalTokens {
	return &ScopeTraversalTokens{
		Root:      Token{Raw: newRawToken(hclsyntax.TokenIdent, root)},
		Traversal: traversers,
	}
}

func (t *ScopeTraversalTokens) GetRoot() Token {
	if t == nil {
		return Token{}
	}
	return t.Root
}

func (t *ScopeTraversalTokens) GetTraversal() []TraverserTokens {
	if t == nil {
		return nil
	}
	return t.Traversal
}

func (*ScopeTraversalTokens) isNodeTokens() {}

// SplatTokens records the tokens associated with an *hclsyntax.SplatExpr.
type SplatTokens struct {
	Open  Token
	Star  Token
	Close *Token
}

func NewSplatTokens(dotted bool) *SplatTokens {
	openType := hclsyntax.TokenDot
	var closeT *Token
	if !dotted {
		openType = hclsyntax.TokenOBrack
		closeT = &Token{Raw: newRawToken(hclsyntax.TokenCBrack)}
	}
	return &SplatTokens{
		Open:  Token{Raw: newRawToken(openType)},
		Star:  Token{Raw: newRawToken(hclsyntax.TokenStar)},
		Close: closeT,
	}
}

func (t *SplatTokens) GetOpen() Token {
	if t == nil {
		return Token{}
	}
	return t.Open
}

func (t *SplatTokens) GetStar() Token {
	if t == nil {
		return Token{}
	}
	return t.Star
}

func (t *SplatTokens) GetClose() *Token {
	if t == nil {
		return nil
	}
	return t.Close
}

func (*SplatTokens) isNodeTokens() {}

// TemplateTokens records the tokens associated with an *hclsyntax.TemplateExpr.
type TemplateTokens struct {
	Open  Token
	Close Token
}

func NewTemplateTokens() *TemplateTokens {
	return &TemplateTokens{
		Open:  Token{Raw: newRawToken(hclsyntax.TokenOQuote)},
		Close: Token{Raw: newRawToken(hclsyntax.TokenCQuote)},
	}
}

func (t *TemplateTokens) GetOpen() Token {
	if t == nil {
		return Token{}
	}
	return t.Open
}

func (t *TemplateTokens) GetClose() Token {
	if t == nil {
		return Token{}
	}
	return t.Close
}

func (*TemplateTokens) isNodeTokens() {}

// TupleConsTokens records the tokens associated with an *hclsyntax.TupleConsExpr.
type TupleConsTokens struct {
	OpenBracket  Token
	Commas       []Token
	CloseBracket Token
}

func NewTupleConsTokens(elementCount int) *TupleConsTokens {
	commas := make([]Token, elementCount-1)
	for i := 0; i < len(commas); i++ {
		commas[i] = Token{Raw: newRawToken(hclsyntax.TokenComma)}
	}
	return &TupleConsTokens{
		OpenBracket:  Token{Raw: newRawToken(hclsyntax.TokenOBrack)},
		Commas:       commas,
		CloseBracket: Token{Raw: newRawToken(hclsyntax.TokenCBrack)},
	}
}

func (t *TupleConsTokens) GetOpenBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.OpenBracket
}

func (t *TupleConsTokens) GetCommas() []Token {
	if t == nil {
		return nil
	}
	return t.Commas
}

func (t *TupleConsTokens) GetCloseBracket() Token {
	if t == nil {
		return Token{}
	}
	return t.CloseBracket
}

func (*TupleConsTokens) isNodeTokens() {}

// UnaryOpTokens records the tokens associated with an *hclsyntax.UnaryOpExpr.
type UnaryOpTokens struct {
	Operator Token
}

func NewUnaryOpTokens(operation *hclsyntax.Operation) *UnaryOpTokens {
	return &UnaryOpTokens{
		Operator: Token{Raw: newRawToken(OperationTokenType(operation))},
	}
}

func (t *UnaryOpTokens) GetOperator() Token {
	if t == nil {
		return Token{}
	}
	return t.Operator
}

func (*UnaryOpTokens) isNodeTokens() {}

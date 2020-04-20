package syntax

import (
	"fmt"
	"math/big"

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

func (trivia TriviaList) LeadingWhitespace() TriviaList {
	end := 0
	for i, t := range trivia {
		if _, ok := t.(Whitespace); !ok {
			break
		}
		end = i
	}
	if end == 0 {
		return nil
	}
	return append(TriviaList(nil), trivia[0:end]...)
}

func (trivia TriviaList) TrailingWhitespace() TriviaList {
	start := len(trivia)
	for i := len(trivia) - 1; i >= 0; i-- {
		if _, ok := trivia[i].(Whitespace); !ok {
			break
		}
		start = i
	}
	if start == len(trivia) {
		return nil
	}
	return append(TriviaList(nil), trivia[start:]...)
}

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

func (t Token) withIdent(s string) Token {
	if string(t.Raw.Bytes) == s {
		return t
	}
	t.Raw.Bytes = []byte(s)
	return t
}

func (t Token) withOperation(operation *hclsyntax.Operation) Token {
	typ := OperationTokenType(operation)
	if t.Raw.Type == typ {
		return t
	}
	t.Raw.Type = typ
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

// Parentheses records enclosing parenthesis tokens for expressions.
type Parentheses struct {
	Open  []Token
	Close []Token
}

func (parens Parentheses) Any() bool {
	return len(parens.Open) > 0
}

func (parens Parentheses) GetLeadingTrivia() TriviaList {
	if !parens.Any() {
		return nil
	}
	return parens.Open[0].LeadingTrivia
}

func (parens Parentheses) SetLeadingTrivia(trivia TriviaList) {
	if parens.Any() {
		parens.Open[0].LeadingTrivia = trivia
	}
}

func (parens Parentheses) GetTrailingTrivia() TriviaList {
	if !parens.Any() {
		return nil
	}
	return parens.Close[0].TrailingTrivia
}

func (parens Parentheses) SetTrailingTrivia(trivia TriviaList) {
	if parens.Any() {
		parens.Close[0].TrailingTrivia = trivia
	}
}

func (parens Parentheses) Format(f fmt.State, c rune) {
	switch c {
	case '(':
		for i := len(parens.Open) - 1; i >= 0; i-- {
			if _, err := fmt.Fprintf(f, "%v", parens.Open[i]); err != nil {
				panic(err)
			}
		}
	case ')':
		for _, p := range parens.Close {
			if _, err := fmt.Fprintf(f, "%v", p); err != nil {
				panic(err)
			}
		}
	default:
		if _, err := fmt.Fprintf(f, "%v%v", parens.Open, parens.Close); err != nil {
			panic(err)
		}
	}
}

func exprRange(filename string, parens Parentheses, start, end hcl.Pos) hcl.Range {
	if parens.Any() {
		start = parens.Open[len(parens.Open)-1].Range().Start
		end = parens.Close[len(parens.Close)-1].Range().End
	}
	return hcl.Range{
		Filename: filename,
		Start:    start,
		End:      end,
	}
}

// AttributeTokens records the tokens associated with an *hclsyntax.Attribute.
type AttributeTokens struct {
	Name   Token
	Equals Token
}

func NewAttributeTokens(name string) *AttributeTokens {
	var t *AttributeTokens
	return &AttributeTokens{
		Name:   t.GetName(name),
		Equals: t.GetEquals(),
	}
}

func (t *AttributeTokens) GetName(name string) Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenIdent, name)}
	}
	return t.Name.withIdent(name)
}

func (t *AttributeTokens) GetEquals() Token {
	if t == nil {
		return Token{
			Raw:           newRawToken(hclsyntax.TokenEqual),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
	}
	return t.Equals
}

func (*AttributeTokens) isNodeTokens() {}

// BinaryOpTokens records the tokens associated with an *hclsyntax.BinaryOpExpr.
type BinaryOpTokens struct {
	Parentheses Parentheses

	Operator Token
}

func NewBinaryOpTokens(operation *hclsyntax.Operation) *BinaryOpTokens {
	var t *BinaryOpTokens
	return &BinaryOpTokens{Operator: t.GetOperator(operation)}
}

func (t *BinaryOpTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *BinaryOpTokens) GetOperator(operation *hclsyntax.Operation) Token {
	if t == nil {
		return Token{
			Raw:           newRawToken(OperationTokenType(operation)),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		}
	}
	return t.Operator.withOperation(operation)
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
	var t *BlockTokens
	return &BlockTokens{
		Type:       t.GetType(typ),
		Labels:     t.GetLabels(labels),
		OpenBrace:  t.GetOpenBrace(),
		CloseBrace: t.GetCloseBrace(),
	}
}

func (t *BlockTokens) GetType(typ string) Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenIdent, typ)}
	}
	return t.Type.withIdent(typ)
}

func (t *BlockTokens) GetLabels(labels []string) []Token {
	if t == nil {
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
		return labelTokens
	}
	return t.Labels
}

func (t *BlockTokens) GetOpenBrace() Token {
	if t == nil {
		return Token{
			Raw:            newRawToken(hclsyntax.TokenOBrace),
			LeadingTrivia:  TriviaList{NewWhitespace(' ')},
			TrailingTrivia: TriviaList{NewWhitespace('\n')},
		}
	}
	return t.OpenBrace
}

func (t *BlockTokens) GetCloseBrace() Token {
	if t == nil {
		return Token{
			Raw:            newRawToken(hclsyntax.TokenCBrace),
			LeadingTrivia:  TriviaList{NewWhitespace('\n')},
			TrailingTrivia: TriviaList{NewWhitespace('\n')},
		}
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
	Parentheses Parentheses

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
	Parentheses Parentheses

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

func (*TemplateForTokens) isNodeTokens() {}

// FunctionCallTokens records the tokens associated with an *hclsyntax.FunctionCallExpr.
type FunctionCallTokens struct {
	Parentheses Parentheses

	Name       Token
	OpenParen  Token
	Commas     []Token
	CloseParen Token
}

func NewFunctionCallTokens(name string, argCount int) *FunctionCallTokens {
	var t *FunctionCallTokens
	return &FunctionCallTokens{
		Name:       t.GetName(name),
		OpenParen:  t.GetOpenParen(),
		Commas:     t.GetCommas(argCount),
		CloseParen: t.GetCloseParen(),
	}
}

func (t *FunctionCallTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *FunctionCallTokens) GetName(name string) Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenIdent, name)}
	}
	return t.Name.withIdent(name)
}

func (t *FunctionCallTokens) GetOpenParen() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenOParen)}
	}
	return t.OpenParen
}

func (t *FunctionCallTokens) GetCommas(argCount int) []Token {
	if t == nil {
		commas := make([]Token, argCount-1)
		for i := 0; i < len(commas); i++ {
			commas[i] = Token{Raw: newRawToken(hclsyntax.TokenComma)}
		}
		return commas
	}
	return t.Commas
}

func (t *FunctionCallTokens) GetCloseParen() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenCParen)}
	}
	return t.CloseParen
}

func (*FunctionCallTokens) isNodeTokens() {}

// IndexTokens records the tokens associated with an *hclsyntax.IndexExpr.
type IndexTokens struct {
	Parentheses Parentheses

	OpenBracket  Token
	CloseBracket Token
}

func NewIndexTokens() *IndexTokens {
	var t *IndexTokens
	return &IndexTokens{
		OpenBracket:  t.GetOpenBracket(),
		CloseBracket: t.GetCloseBracket(),
	}
}

func (t *IndexTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *IndexTokens) GetOpenBracket() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenOBrack)}
	}
	return t.OpenBracket
}

func (t *IndexTokens) GetCloseBracket() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenCBrack)}
	}
	return t.CloseBracket
}

func (*IndexTokens) isNodeTokens() {}

// LiteralValueTokens records the tokens associated with an *hclsyntax.LiteralValueExpr.
type LiteralValueTokens struct {
	Parentheses Parentheses

	Value []Token
}

func rawLiteralValueToken(value cty.Value) hclsyntax.Token {
	rawType, rawText := hclsyntax.TokenIdent, ""
	switch value.Type() {
	case cty.Bool:
		rawText = "false"
		if value.True() {
			rawText = "true"
		}
	case cty.Number:
		rawType = hclsyntax.TokenNumberLit

		bf := value.AsBigFloat()
		i, acc := bf.Int64()
		if acc == big.Exact {
			rawText = fmt.Sprintf("%v", i)
		} else {
			d, _ := bf.Float64()
			rawText = fmt.Sprintf("%g", d)
		}
	case cty.String:
		rawText = value.AsString()
	}
	return newRawToken(rawType, rawText)
}

func NewLiteralValueTokens(value cty.Value) *LiteralValueTokens {
	var t *LiteralValueTokens
	return &LiteralValueTokens{
		Value: t.GetValue(value),
	}
}

func (t *LiteralValueTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *LiteralValueTokens) GetValue(value cty.Value) []Token {
	if t == nil {
		return []Token{{Raw: rawLiteralValueToken(value)}}
	}
	return t.Value
}

func (*LiteralValueTokens) isNodeTokens() {}

// ObjectConsItemTokens records the tokens associated with an hclsyntax.ObjectConsItem.
type ObjectConsItemTokens struct {
	Equals Token
	Comma  *Token
}

func NewObjectConsItemTokens(last bool) ObjectConsItemTokens {
	var comma *Token
	if !last {
		comma = &Token{
			Raw:            newRawToken(hclsyntax.TokenComma),
			TrailingTrivia: TriviaList{NewWhitespace('\n')},
		}

	}
	return ObjectConsItemTokens{
		Equals: Token{
			Raw:           newRawToken(hclsyntax.TokenEqual),
			LeadingTrivia: TriviaList{NewWhitespace(' ')},
		},
		Comma: comma,
	}
}

// ObjectConsTokens records the tokens associated with an *hclsyntax.ObjectConsExpr.
type ObjectConsTokens struct {
	Parentheses Parentheses

	OpenBrace  Token
	Items      []ObjectConsItemTokens
	CloseBrace Token
}

func NewObjectConsTokens(itemCount int) *ObjectConsTokens {
	var t *ObjectConsTokens
	return &ObjectConsTokens{
		OpenBrace:  t.GetOpenBrace(itemCount),
		Items:      t.GetItems(itemCount),
		CloseBrace: t.GetCloseBrace(),
	}
}

func (t *ObjectConsTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *ObjectConsTokens) GetOpenBrace(itemCount int) Token {
	if t == nil {
		var openBraceTrailingTrivia TriviaList
		if itemCount > 0 {
			openBraceTrailingTrivia = TriviaList{NewWhitespace('\n')}
		}
		return Token{
			Raw:            newRawToken(hclsyntax.TokenOBrace),
			TrailingTrivia: openBraceTrailingTrivia,
		}
	}
	return t.OpenBrace
}

func (t *ObjectConsTokens) GetItems(itemCount int) []ObjectConsItemTokens {
	if t == nil {
		items := make([]ObjectConsItemTokens, itemCount)
		for i := 0; i < len(items); i++ {
			items[i] = NewObjectConsItemTokens(i == len(items)-1)
		}
		return items
	}
	return t.Items
}

func (t *ObjectConsTokens) GetCloseBrace() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenCBrace)}
	}
	return t.CloseBrace
}

func (*ObjectConsTokens) isNodeTokens() {}

// TraverserTokens is a closed interface implemented by DotTraverserTokens and BracketTraverserTokens
type TraverserTokens interface {
	Range() hcl.Range

	isTraverserTokens()
}

func NewTraverserTokens(traverser hcl.Traverser) TraverserTokens {
	switch traverser := traverser.(type) {
	case hcl.TraverseAttr:
		return NewDotTraverserTokens(traverser.Name)
	case hcl.TraverseIndex:
		return NewBracketTraverserTokens(string(rawLiteralValueToken(traverser.Key).Bytes))
	default:
		return nil
	}
}

// DotTraverserTokens records the tokens associated with dotted traverser (i.e. '.' <attr>).
type DotTraverserTokens struct {
	Parentheses Parentheses

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

func (t *DotTraverserTokens) Range() hcl.Range {
	filename := t.Dot.Range().Filename
	return exprRange(filename, t.Parentheses, t.Dot.Range().Start, t.Index.Range().End)
}

func (*DotTraverserTokens) isTraverserTokens() {}

// BracketTraverserTokens records the tokens associated with a bracketed traverser (i.e. '[' <index> ']').
type BracketTraverserTokens struct {
	Parentheses Parentheses

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

func (t *BracketTraverserTokens) Range() hcl.Range {
	filename := t.OpenBracket.Range().Filename
	return exprRange(filename, t.Parentheses, t.OpenBracket.Range().Start, t.CloseBracket.Range().End)
}

func (*BracketTraverserTokens) isTraverserTokens() {}

func newRelativeTraversalTokens(traversal hcl.Traversal) []TraverserTokens {
	result := make([]TraverserTokens, len(traversal))
	for i, t := range traversal {
		result[i] = NewTraverserTokens(t)
	}
	return result
}

// RelativeTraversalTokens records the tokens associated with an *hclsyntax.RelativeTraversalExpr.
type RelativeTraversalTokens struct {
	Parentheses Parentheses

	Traversal []TraverserTokens
}

func NewRelativeTraversalTokens(traversal hcl.Traversal) *RelativeTraversalTokens {
	return &RelativeTraversalTokens{
		Traversal: newRelativeTraversalTokens(traversal),
	}
}

func (t *RelativeTraversalTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *RelativeTraversalTokens) GetTraversal(traversal hcl.Traversal) []TraverserTokens {
	if t == nil {
		return newRelativeTraversalTokens(traversal)
	}
	return t.Traversal
}

func (*RelativeTraversalTokens) isNodeTokens() {}

// ScopeTraversalTokens records the tokens associated with an *hclsyntax.ScopeTraversalExpr.
type ScopeTraversalTokens struct {
	Parentheses Parentheses

	Root      Token
	Traversal []TraverserTokens
}

func NewScopeTraversalTokens(traversal hcl.Traversal) *ScopeTraversalTokens {
	var t *ScopeTraversalTokens
	return &ScopeTraversalTokens{
		Root:      t.GetRoot(traversal),
		Traversal: t.GetTraversal(traversal),
	}
}

func (t *ScopeTraversalTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *ScopeTraversalTokens) GetRoot(traversal hcl.Traversal) Token {
	if t == nil {
		rootName := traversal[0].(hcl.TraverseRoot).Name
		return Token{Raw: newRawToken(hclsyntax.TokenIdent, rootName)}
	}
	return t.Root
}

func (t *ScopeTraversalTokens) GetTraversal(traversal hcl.Traversal) []TraverserTokens {
	if t == nil {
		return newRelativeTraversalTokens(traversal[1:])
	}
	return t.Traversal
}

func (*ScopeTraversalTokens) isNodeTokens() {}

// SplatTokens records the tokens associated with an *hclsyntax.SplatExpr.
type SplatTokens struct {
	Parentheses Parentheses

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

func (t *SplatTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *SplatTokens) GetOpen() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenOBrack)}
	}
	return t.Open
}

func (t *SplatTokens) GetStar() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenStar)}
	}
	return t.Star
}

func (t *SplatTokens) GetClose() *Token {
	if t == nil {
		return &Token{Raw: newRawToken(hclsyntax.TokenCBrack)}
	}
	return t.Close
}

func (*SplatTokens) isNodeTokens() {}

// TemplateTokens records the tokens associated with an *hclsyntax.TemplateExpr.
type TemplateTokens struct {
	Parentheses Parentheses

	Open  Token
	Close Token
}

func NewTemplateTokens() *TemplateTokens {
	var t *TemplateTokens
	return &TemplateTokens{
		Open:  t.GetOpen(),
		Close: t.GetClose(),
	}
}

func (t *TemplateTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *TemplateTokens) GetOpen() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenOQuote)}
	}
	return t.Open
}

func (t *TemplateTokens) GetClose() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenCQuote)}
	}
	return t.Close
}

func (*TemplateTokens) isNodeTokens() {}

// TupleConsTokens records the tokens associated with an *hclsyntax.TupleConsExpr.
type TupleConsTokens struct {
	Parentheses Parentheses

	OpenBracket  Token
	Commas       []Token
	CloseBracket Token
}

func NewTupleConsTokens(elementCount int) *TupleConsTokens {
	var t *TupleConsTokens
	return &TupleConsTokens{
		OpenBracket:  t.GetOpenBracket(),
		Commas:       t.GetCommas(elementCount),
		CloseBracket: t.GetCloseBracket(),
	}
}

func (t *TupleConsTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *TupleConsTokens) GetOpenBracket() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenOBrack)}
	}
	return t.OpenBracket
}

func (t *TupleConsTokens) GetCommas(elementCount int) []Token {
	if t == nil {
		commas := make([]Token, elementCount-1)
		for i := 0; i < len(commas); i++ {
			commas[i] = Token{Raw: newRawToken(hclsyntax.TokenComma)}
		}
		return commas
	}
	return t.Commas
}

func (t *TupleConsTokens) GetCloseBracket() Token {
	if t == nil {
		return Token{Raw: newRawToken(hclsyntax.TokenCBrack)}
	}
	return t.CloseBracket
}

func (*TupleConsTokens) isNodeTokens() {}

// UnaryOpTokens records the tokens associated with an *hclsyntax.UnaryOpExpr.
type UnaryOpTokens struct {
	Parentheses Parentheses

	Operator Token
}

func NewUnaryOpTokens(operation *hclsyntax.Operation) *UnaryOpTokens {
	var t *UnaryOpTokens
	return &UnaryOpTokens{
		Operator: t.GetOperator(operation),
	}
}

func (t *UnaryOpTokens) GetParentheses() Parentheses {
	if t == nil {
		return Parentheses{}
	}
	return t.Parentheses
}

func (t *UnaryOpTokens) GetOperator(operation *hclsyntax.Operation) Token {
	if t == nil {
		return Token{Raw: newRawToken(OperationTokenType(operation))}
	}
	return t.Operator
}

func (*UnaryOpTokens) isNodeTokens() {}

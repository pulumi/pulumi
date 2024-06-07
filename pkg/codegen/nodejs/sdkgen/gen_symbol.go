package sdkgen

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs/codebase"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type ResourceSymbols struct {
	Article   string
	Type      codebase.NamedSymbol
	StateType codebase.NamedSymbol
	ArgsType  codebase.NamedSymbol
}

func (g *generator) makeResourceSymbols(r *schema.Resource) (ResourceSymbols, error) {
	parsed, err := g.parseToken(r.Token)
	if err != nil {
		return ResourceSymbols{}, err
	}

	stateTypeName := parsed.TypeName + "State"
	argsTypeName := parsed.TypeName + "Args"

	article := "a"
	beginsWithVowel := strings.HasPrefix(parsed.TypeName, "A") ||
		strings.HasPrefix(parsed.TypeName, "E") ||
		strings.HasPrefix(parsed.TypeName, "I") ||
		strings.HasPrefix(parsed.TypeName, "O") ||
		strings.HasPrefix(parsed.TypeName, "U")

	if beginsWithVowel {
		article = "an"
	}

	typeSym := codebase.NamedSymbol{
		Module: parsed.Module,
		Name:   parsed.TypeName,
		As:     "",
	}

	stateTypeSym := codebase.NamedSymbol{
		Module: parsed.Module,
		Name:   stateTypeName,
		As:     "",
	}

	argsTypeSym := codebase.NamedSymbol{
		Module: parsed.Module,
		Name:   argsTypeName,
		As:     "",
	}

	return ResourceSymbols{
		Article:   article,
		Type:      typeSym,
		StateType: stateTypeSym,
		ArgsType:  argsTypeSym,
	}, nil
}

type PlainTypeSymbols struct {
	Type codebase.NamedSymbol
}

func (g *generator) makePlainTypeSymbols(token string) (PlainTypeSymbols, error) {
	parsed, err := g.parseToken(token)
	if err != nil {
		return PlainTypeSymbols{}, err
	}

	typeSym := codebase.NamedSymbol{
		Module: parsed.Module,
		Name:   parsed.TypeName,
		As:     "",
	}

	return PlainTypeSymbols{
		Type: typeSym,
	}, nil
}

type ParsedToken struct {
	Token string
	// slashed, suitable for import
	Module string
	// pascal cased, suitable for type names
	TypeName string
}

// token example:
// aws:accessanalyzer/AnalyzerConfigurationUnusedAccess:AnalyzerConfigurationUnusedAccess
func (g *generator) parseToken(token string) (ParsedToken, error) {
	moduleBase := g.pkg.TokenToModule(token)
	if moduleOverride, ok := g.info.ModuleToPackage[moduleBase]; ok {
		moduleBase = moduleOverride
	}

	tokenParts := strings.Split(token, ":")
	if len(tokenParts) != 3 {
		return ParsedToken{}, fmt.Errorf(
			"failed to parse token %s: expected 3 parts but got %d",
			token, len(tokenParts),
		)
	}

	moduleName := camelCase(tokenParts[2])
	if isReservedModuleName(moduleName) {
		moduleName = moduleName + "_"
	}

	module := moduleBase + "/" + moduleName
	typeName := titleCase(moduleName)

	return ParsedToken{
		Token:    token,
		Module:   module,
		TypeName: typeName,
	}, nil
}

func isReservedModuleName(name string) bool {
	if name == "index" {
		return true
	}

	return false
}

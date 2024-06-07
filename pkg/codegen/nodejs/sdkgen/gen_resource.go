package sdkgen

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs/codebase"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func (g *generator) generateResource(r *schema.Resource) error {
	rSyms, err := g.makeResourceSymbols(r)
	if err != nil {
		return err
	}

	m := g.codebase.Module(rSyms.Type.Module)

	err = generateResourceClass(g, r, m, rSyms)
	if err != nil {
		return err
	}

	err = generateResourceStateInterface(g, r, m, rSyms)
	if err != nil {
		return err
	}

	err = generateResourceArgsInterface(g, r, m, rSyms)
	if err != nil {
		return err
	}

	return nil
}

func generateResourceClass(
	g *generator,
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
) error {
	rClass := m.Class(
		[]codebase.Modifier{codebase.Export},
		rSyms.Type.Name,
		[]codebase.TypeParameter{},
	).Documented(
		codebase.TD(commentTypeDoc(r.Comment)).Deprecated(r.DeprecationMessage),
	)

	var rBaseType, rOptionsType codebase.Type

	switch {
	case r.IsProvider:
		rBaseType = m.QualifiedSymbolImport(PProviderResource).AsType()
		rOptionsType = m.QualifiedSymbolImport(PResourceOptions).AsType()
	case r.IsComponent:
		rBaseType = m.QualifiedSymbolImport(PComponentResource).AsType()
		rOptionsType = m.QualifiedSymbolImport(PComponentResourceOptions).AsType()
	default:
		rBaseType = m.QualifiedSymbolImport(PCustomResource).AsType()
		rOptionsType = m.QualifiedSymbolImport(PCustomResourceOptions).AsType()
	}

	rClass.Extends(rBaseType)

	generateResourceClassGetMethod(r, m, rSyms, rClass, rOptionsType)
	generateResourceClassPulumiTypeProperty(g, r, rClass)
	generateResourceClassIsInstanceMethod(r, m, rSyms, rClass)

	genPs, err := generateResourceClassProperties(g, r, m, rSyms, rClass)
	if err != nil {
		return err
	}

	generateResourceClassConstructor(r, m, rSyms, rClass, genPs)

	return nil
}

func generateResourceClassGetMethod(
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
	rClass *codebase.Class,
	rOptionsType codebase.Type,
) {
	if !r.IsProvider && !r.IsComponent {
		getTypeDoc := codebase.TD(fmt.Sprintf(
			`Get an existing %s resource's state with the given name, ID, and optional extra
properties used to qualify the lookup.`,
			rSyms.Type.Name,
		)).
			Parameter("name", "The _unique_ name of the resulting resource").
			Parameter("id", "The _unique_ provider ID of the resource to lookup").
			Parameter("state", "Any extra arguments used during the lookup.").
			Parameter("opts", "Optional settings to control the behaviour of the CustomResource.")

		nameArgName := "name"
		idArgName := "id"
		getArguments := []codebase.Argument{
			{
				Name: nameArgName,
				Type: codebase.StringT,
			},
			{
				Name: idArgName,
				Type: InputT(m, m.QualifiedSymbolImport(PID).AsType()),
			},
		}

		stateRef := codebase.AsE(codebase.UndefinedE, codebase.AnyT)
		if r.StateInputs != nil {
			stateArgName := "state"
			stateRef = codebase.AsE(codebase.RefE(stateArgName), codebase.AnyT)

			getArguments = append(
				getArguments,
				codebase.Argument{
					Modifiers: []codebase.Modifier{codebase.Optional},
					Name:      stateArgName,
					Type:      m.NamedSymbolImport(rSyms.StateType).AsType(),
				},
			)
		}

		optsArgName := "opts"
		getArguments = append(
			getArguments,
			codebase.Argument{
				Modifiers: []codebase.Modifier{codebase.Optional},
				Name:      optsArgName,
				Type:      rOptionsType,
			},
		)

		rClass.Method(
			[]codebase.Modifier{codebase.Public, codebase.Static},
			"get",
			getArguments,
			rSyms.Type.AsType(),
			[]codebase.Statement{
				codebase.ReturnS(
					codebase.NewE(
						rSyms.Type.AsType(),
						codebase.RefE(nameArgName),
						stateRef,
						codebase.ObjectE(
							codebase.SpreadOP(codebase.RefE(optsArgName)),
							codebase.StringKeyOP("id", codebase.RefE(idArgName)),
						),
					),
				),
			},
		).Documented(getTypeDoc)
	}
}

func generateResourceClassPulumiTypeProperty(
	g *generator,
	r *schema.Resource,
	rClass *codebase.Class,
) {
	pulumiType := r.Token
	if r.IsProvider {
		pulumiType = g.pkg.Name
	}

	rClass.Property(
		[]codebase.Modifier{codebase.Public, codebase.Static, codebase.Readonly},
		"__pulumiType",
		codebase.StringT,
	).
		Initialized(codebase.StringE(pulumiType)).
		Documented(codebase.TD("").Internal())
}

func generateResourceClassIsInstanceMethod(
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
	rClass *codebase.Class,
) {
	objArgName := "obj"
	objArgRef := codebase.RefE(objArgName)

	typeExpr := rSyms.Type.AsExpression().Property("__pulumiType")
	if r.IsProvider {
		typeExpr = codebase.AddE(
			codebase.StringE("pulumi:providers:"),
			typeExpr,
		)
	}

	rClass.Method(
		[]codebase.Modifier{codebase.Public, codebase.Static},
		"isInstance",
		[]codebase.Argument{
			{
				Name: "obj",
				Type: codebase.AnyT,
			},
		},
		codebase.GuardT("obj", rSyms.Type.AsType()),
		[]codebase.Statement{
			codebase.IfS(
				codebase.OrE(
					codebase.EqualE(objArgRef, codebase.UndefinedE),
					codebase.EqualE(objArgRef, codebase.NullE),
				),
				[]codebase.Statement{
					codebase.ReturnS(codebase.FalseE),
				},
				[]codebase.Statement{},
			),

			codebase.ReturnS(
				codebase.EqualE(
					objArgRef.Index(codebase.StringE("__pulumiType")),
					typeExpr,
				),
			),
		},
	)
}

type GeneratedResourceClassProperties struct {
	AllInputsOptional bool
}

func generateResourceClassProperties(
	g *generator,
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
	rClass *codebase.Class,
) (GeneratedResourceClassProperties, error) {
	inputs := mapset.NewSet[string]()
	allInputsOptional := true
	for _, p := range r.InputProperties {
		inputs.Add(p.Name)
		if p.IsRequired() {
			allInputsOptional = false
		}
	}

	for _, p := range r.Properties {
		modifiers := []codebase.Modifier{
			codebase.Public,
			codebase.Readonly,
			codebase.Required,
		}

		if !inputs.Contains(p.Name) {
			modifiers = append(modifiers, codebase.Output)
		}

		pt := p.Type
		if g.info.Compatibility == kubernetes20 {
			pt = ensureRequiredType(pt)
		}

		t, err := g.generateType(m, pt)
		if err != nil {
			return GeneratedResourceClassProperties{}, err
		}

		rClass.Property(
			modifiers,
			p.Name,
			OutputT(m, t),
		).Documented(codebase.TD(commentTypeDoc(p.Comment)).Deprecated(p.DeprecationMessage))
	}

	return GeneratedResourceClassProperties{AllInputsOptional: allInputsOptional}, nil
}

func generateResourceClassConstructor(
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
	rClass *codebase.Class,
	genPs GeneratedResourceClassProperties,
) {
}

func generateResourceStateInterface(
	g *generator,
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
) error {
	if r.StateInputs != nil {
		stateInterface := m.Interface(
			[]codebase.Modifier{codebase.Export},
			rSyms.StateType.Name,
			[]codebase.TypeParameter{},
		).Documented(
			codebase.TD(commentTypeDoc(r.StateInputs.Comment)),
		)

		for _, p := range r.StateInputs.Properties {
			modifiers, pt := propertyTypeAndModifiers(p)
			pType, err := g.generateType(m, pt)
			if err != nil {
				return err
			}

			stateInterface.Property(
				modifiers,
				p.Name,
				pType,
			).Documented(
				codebase.TD(commentTypeDoc(p.Comment)).Deprecated(p.DeprecationMessage),
			)
		}
	}

	return nil
}

func generateResourceArgsInterface(
	g *generator,
	r *schema.Resource,
	m *codebase.Module,
	rSyms ResourceSymbols,
) error {
	if r.InputProperties != nil {
		argsInterface := m.Interface(
			[]codebase.Modifier{codebase.Export},
			rSyms.ArgsType.Name,
			[]codebase.TypeParameter{},
		).Documented(
			codebase.TD(fmt.Sprintf(
				"The set of arguments for constructing %s %s resource.",
				rSyms.Article, rSyms.Type.Name,
			)),
		)

		for _, p := range r.InputProperties {
			modifiers, pt := propertyTypeAndModifiers(p)
			pType, err := g.generateType(m, pt)
			if err != nil {
				return err
			}

			argsInterface.Property(
				modifiers,
				p.Name,
				pType,
			).Documented(
				codebase.TD(commentTypeDoc(p.Comment)).Deprecated(p.DeprecationMessage),
			)
		}
	}

	return nil
}

func propertyTypeAndModifiers(p *schema.Property) ([]codebase.Modifier, schema.Type) {
	if p.IsRequired() {
		return []codebase.Modifier{}, p.Type
	}

	switch pt := p.Type.(type) {
	case *schema.OptionalType:
		return []codebase.Modifier{codebase.Optional}, pt.ElementType
	default:
		return []codebase.Modifier{}, p.Type
	}
}

func ensureOptionalType(t schema.Type) schema.Type {
	if _, ok := t.(*schema.OptionalType); ok {
		return t
	}

	return &schema.OptionalType{ElementType: t}
}

func ensureRequiredType(t schema.Type) schema.Type {
	if _, ok := t.(*schema.OptionalType); ok {
		return t.(*schema.OptionalType).ElementType
	}

	return t
}

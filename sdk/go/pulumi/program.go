package pulumi

import (
	"reflect"
)

type Program struct {
	CustomResourceState

	// Name is in the form "Org/Program/Stack"
	Name StringOutput `pulumi:"name"`
	// Outputs resolves with exports from the named stack
	Outputs MapOutput `pulumi:"outputs"`
}

type programArgs struct {
	Name                string                 `pulumi:"name"`
	Source              string                 `pulumi:"source"`
	Inputs              map[string]interface{} `pulumi:"inputs"`
	PrefixResourceNames bool                   `pulumi:"prefixResourceNames"`
}

type ProgramArgs struct {
	// Name is the unique name of the resource.
	Name StringInput

	// Source is the source of the program to execute.
	Source StringInput

	// Inputs is the input to configure the stack
	Inputs MapInput

	// PrefixResourceNames is a boolean that indicates whether to prefix the names of resources with the name of the program.
	PrefixResourceNames BoolInput
}

func (ProgramArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*programArgs)(nil)).Elem()
}

func NewProgram(ctx *Context, name string, args *ProgramArgs, opts ...ResourceOption) (*Program, error) {
	if args == nil {
		args = &ProgramArgs{}
	}
	if args.Name == nil {
		args.Name = StringInput(String(name))
	}

	prog := Program{}
	if err := ctx.RegisterResource("pulumi:pulumi:Program", name, args, &prog, opts...); err != nil {
		return nil, err
	}
	return &prog, nil
}

func (p *Program) GetOutputs() MapOutput {
	return p.Outputs
}

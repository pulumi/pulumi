package trees

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/x/provider"
)

type ClayPotArgs struct {
	// Size indicates the size of the pot.
	Size string `pulumi:"size,immutable"`
}

//pulumi:resource
type ClayPot struct {
	// Size indicates the size of the pot.
	Size string `pulumi:"size,immutable"`
}

func (m *ClayPot) Args() *ClayPotArgs {
	return &ClayPotArgs{Size: m.Size}
}

func (m *ClayPot) Create(ctx *provider.Context, p *landscape.Provider, args *ClayPotArgs, options provider.CreateOptions) (provider.ID, error) {
	m.Size = args.Size
	return string(m.Size), nil
}

func (m *ClayPot) Read(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.ReadOptions) error {
	if m.Size != "" && m.Size != string(id) {
		return provider.NotFound(fmt.Sprintf("container %v does not exist", id))
	}

	m.Size = string(id)
	return nil
}

func (m *ClayPot) Update(ctx *provider.Context, p *landscape.Provider, id provider.ID, args *ClayPotArgs, options provider.UpdateOptions) error {
	return nil
}

func (m *ClayPot) Delete(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.DeleteOptions) error {
	return nil
}

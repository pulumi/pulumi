package trees

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/provider"
)

type OakTreeArgs struct {
	// Species indicates the species of oak.
	Species string `pulumi:"species,immutable"`
	// Container indicates the container, if any, that holds this tree.
	Container string `pulumi:"container"`
}

//pulumi:resource
type OakTree struct {
	// Species indicates the species of oak.
	Species string `pulumi:"species,immutable"`
	// Container indicates the container, if any, that holds this tree.
	Container string `pulumi:"container"`
	// PlantedOn indicates the date the tree was planted.
	PlantedOn string `pulumi:"plantedOn,immutable"`
	// Age indicates the age of the oak tree in days.
	Age float64 `pulumi:"age"`
}

func (m *OakTree) Args() *OakTreeArgs {
	return &OakTreeArgs{
		Species:   m.Species,
		Container: m.Container,
	}
}

func (m *OakTree) Create(ctx *provider.Context, p *landscape.Provider, args *OakTreeArgs, options provider.CreateOptions) (provider.ID, error) {
	m.Species = args.Species
	m.Container = args.Container
	m.PlantedOn = time.Now().UTC().Format(time.RFC3339)
	m.Age = 0
}

func (m *OakTree) Read(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.ReadOptions) error {
	if m.Species == "" || m.PlantedOn == 0 {
		return provider.NotFound(fmt.Sprintf("tree %v does not exist", id))
	}

	plantedOn, err := time.Parse(time.RFC3339, m.PlantedOn)
	if err != nil {
		return fmt.Errorf("malformed originiation date '%v': %v", m.PlantedOn, err)
	}

	m.Age = time.Since(plantedOn).Hours() / 24
	return nil
}

func (m *OakTree) Update(ctx *provider.Context, p *landscape.Provider, id provider.ID, args *OakTreeArgs, options provider.UpdateOptions) error {
	m.Container = args.Container
	return nil
}

func (m *OakTree) Delete(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.DeleteOptions) error {
	return nil
}

package trees

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/x/provider"
)

type CedarTreeArgs struct {
	// Species indicates the species of cedar.
	Species string `pulumi:"species,immutable"`
	// Container indicates the container, if any, that holds this tree.
	Container string `pulumi:"container"`
}

//pulumi:resource
type CedarTree struct {
	// Species indicates the species of cedar.
	Species string `pulumi:"species,immutable"`
	// Container indicates the container, if any, that holds this tree.
	Container string `pulumi:"container"`
	// PlantedOn indicates the date the tree was planted.
	PlantedOn string `pulumi:"plantedOn,immutable"`
	// Age indicates the age of the cedar tree in days.
	Age float64 `pulumi:"age"`
}

func (m *CedarTree) Args() *CedarTreeArgs {
	return &CedarTreeArgs{
		Species:   m.Species,
		Container: m.Container,
	}
}

func (m *CedarTree) Create(ctx *provider.Context, p *landscape.Provider, args *CedarTreeArgs, options provider.CreateOptions) (provider.ID, error) {
	m.Species = args.Species
	m.Container = args.Container
	m.PlantedOn = time.Now().UTC().Format(time.RFC3339)
	m.Age = 0
}

func (m *CedarTree) Read(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.ReadOptions) error {
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

func (m *CedarTree) Update(ctx *provider.Context, p *landscape.Provider, id provider.ID, args *CedarTreeArgs, options provider.UpdateOptions) error {
	m.Container = args.Container
	return nil
}

func (m *CedarTree) Delete(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.DeleteOptions) error {
	return nil
}

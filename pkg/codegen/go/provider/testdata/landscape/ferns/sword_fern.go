package ferns

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/x/provider"
)

type SwordFernArgs struct {
	// Container indicates the container, if any, that holds this fern.
	Container string `pulumi:"container"`
}

//pulumi:resource
type SwordFern struct {
	// Container indicates the container, if any, that holds this fern.
	Container string `pulumi:"container"`
	// PlantedOn indicates the date the fern was planted.
	PlantedOn string `pulumi:"plantedOn,immutable"`
	// Age indicates the age of the cedar fern in days.
	Age float64 `pulumi:"age"`
}

func (m *SwordFern) Args() *SwordFernArgs {
	return &SwordFernArgs{Container: m.Container}
}

func (m *SwordFern) Create(ctx *provider.Context, p *landscape.Provider, args *SwordFernArgs, options provider.CreateOptions) (provider.ID, error) {
	m.Container = args.Container
	m.PlantedOn = time.Now().UTC().Format(time.RFC3339)
	m.Age = 0
}

func (m *SwordFern) Read(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.ReadOptions) error {
	if m.PlantedOn == 0 {
		return provider.NotFound(fmt.Sprintf("fern %v does not exist", id))
	}

	plantedOn, err := time.Parse(time.RFC3339, m.PlantedOn)
	if err != nil {
		return fmt.Errorf("malformed originiation date '%v': %v", m.PlantedOn, err)
	}

	m.Age = time.Since(plantedOn).Hours() / 24
	return nil
}

func (m *SwordFern) Update(ctx *provider.Context, p *landscape.Provider, id provider.ID, args *SwordFernArgs, options provider.UpdateOptions) error {
	m.Container = args.Container
	return nil
}

func (m *SwordFern) Delete(ctx *provider.Context, p *landscape.Provider, id provider.ID, options provider.DeleteOptions) error {
	return nil
}

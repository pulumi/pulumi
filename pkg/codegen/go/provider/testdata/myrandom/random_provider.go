package myrandom

import (
	"math/rand"
	"sync"
)

type ProviderArgs struct {
	// The seed for the provider's PRNG, if any.
	Seed *int `pulumi:"seed"`
}

//pulumi:provider
type Provider struct {
	m    sync.Mutex
	rand *rand.Rand
}

func (p *Provider) Configure(ctx *provider.Context, args *ProviderArgs, options provider.ConfigureOptions) error {
	seed := 0
	if args.Seed != nil {
		seed = *args.Seed
	}

	p.rand = rand.New(rand.NewSource(int64(seed)))
	return nil
}

func (p *Provider) randomBytes(count int) byte {
	p.m.Lock()
	defer p.m.Unlock()

	bytes := make([]byte, count)
	for i := range bytes {
		bytes[i] = byte(p.rand.Int31n(256))
	}
	return bytes
}

// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

type ProviderGenerator struct {
	Root string
	Out  string
}

func NewProviderGenerator(root string, out string) *ProviderGenerator {
	return &ProviderGenerator{
		Root: root,
		Out:  out,
	}
}

func (pg *ProviderGenerator) Generate(pkg *Package) error {
	return nil
}

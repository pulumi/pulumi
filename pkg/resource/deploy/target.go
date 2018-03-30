// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Target represents information about a deployment target.
type Target struct {
	Name      tokens.QName     // the target stack name.
	Config    config.Map       // optional configuration key/value pairs.
	Decrypter config.Decrypter // decrypter for secret configuration values.
	Snapshot  *Snapshot        // the last snapshot deployed to the target.
}

// GetPackageConfig returns the set of configuration parameters for the indicated package, if any.
func (t *Target) GetPackageConfig(pkg tokens.Package) (map[config.Key]string, error) {
	var result map[config.Key]string
	for k, c := range t.Config {
		if tokens.Package(k.Namespace()) != pkg {
			continue
		}
		v, err := c.Value(t.Decrypter)
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = make(map[config.Key]string)
		}
		result[k] = v
	}
	return result, nil
}

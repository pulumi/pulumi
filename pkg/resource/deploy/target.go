// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
}

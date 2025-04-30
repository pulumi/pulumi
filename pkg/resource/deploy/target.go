// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Target represents information about a deployment target.
type Target struct {
	Name         tokens.StackName   // the target stack name.
	Project      tokens.PackageName // the target project name.
	Organization tokens.Name        // the target organization name (if any).
	Config       config.Map         // optional configuration key/value pairs.
	Decrypter    config.Decrypter   // decrypter for secret configuration values.
	Snapshot     *Snapshot          // the last snapshot deployed to the target.
}

const (
	// The package name for the NodeJS dynamic provider.
	nodejsDynamicProviderPackage = "pulumi-nodejs"
	// The package name for the Python dynamic provider.
	pythonDynamicProviderPackage = "pulumi-python"
)

// Returns true if the given package refers to a dynamic provider.
func isDynamicProvider(pkg tokens.Package) bool {
	return pkg == tokens.Package(nodejsDynamicProviderPackage) || pkg == tokens.Package(pythonDynamicProviderPackage)
}

func (t *Target) StackReference() resource.AbsoluteStackReference {
	return resource.AbsoluteStackReference{
		Stack:        t.Name.String(),
		Project:      t.Project.String(),
		Organization: t.Organization.String(),
	}
}

// GetPackageConfig returns the set of configuration parameters for the indicated package, if any.
func (t *Target) GetPackageConfig(pkg tokens.Package) (resource.PropertyMap, error) {
	result := resource.PropertyMap{}
	if t == nil {
		return result, nil
	}

	for k, c := range t.Config {
		// For dynamic providers, we always pass the full configuration.
		if !isDynamicProvider(pkg) && tokens.Package(k.Namespace()) != pkg {
			continue
		}

		v, err := c.Value(t.Decrypter)
		if err != nil {
			return nil, err
		}

		propertyValue := resource.NewStringProperty(v)
		if c.Secure() {
			propertyValue = resource.MakeSecret(propertyValue)
		}
		if isDynamicProvider(pkg) {
			// For dynamic providers, we want to maintain the namespace.
			result[resource.PropertyKey(k.String())] = propertyValue
		} else {
			result[resource.PropertyKey(k.Name())] = propertyValue
		}
	}
	return result, nil
}

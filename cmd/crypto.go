// Copyright 2016-2019, Pulumi Corporation.
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
package cmd

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/resource/config"
)

func getStackCrypter(s backend.Stack) (config.Crypter, error) {
	switch stack := s.(type) {
	case httpstate.Stack:
		return newCloudCrypter(stack.Backend().(httpstate.Backend).Client(), stack.StackIdentifier()), nil
	case filestate.Stack:
		return symmetricCrypter(s.Ref().Name(), stackConfigFile)
	}

	return nil, errors.Errorf("unknown stack type %s", reflect.TypeOf(s))
}

// Copyright 2016-2022, Pulumi Corporation.
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

// A small library for creating consistent and documented environmental variable accesses.
//
// Public environmental variables should be declared as a module level variable.

package env

import "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"

// Re-export some types and functions from the env library.

type Env = env.Env

type MapStore = env.MapStore

func NewEnv(s env.Store) env.Env { return env.NewEnv(s) }

// Global is the environment defined by environmental variables.
func Global() env.Env {
	return env.NewEnv(env.Global)
}

// That Pulumi is running in experimental mode.
//
// This is our standard gate for an existing feature that's not quite ready to be stable
// and publicly consumed.
var Experimental = env.Bool("EXPERIMENTAL", "Enable experimental options and commands")

var SkipUpdateCheck = env.Bool("SKIP_UPDATE_CHECK", "Disable checking for a new version of pulumi")

var Dev = env.Bool("DEV", "Enable features for hacking on pulumi itself")

var IgnoreAmbientPlugins = env.Bool("IGNORE_AMBIENT_PLUGINS",
	"Discover additional plugins by examining the $PATH")

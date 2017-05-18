// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pack

import (
	"sort"

	"github.com/pulumi/lumi/pkg/tokens"
)

func StableDependencies(deps Dependencies) []tokens.PackageName {
	sorted := make(packageNames, 0, len(deps))
	for dep := range deps {
		sorted = append(sorted, dep)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleAliases(aliases ModuleAliases) []tokens.ModuleName {
	sorted := make(moduleNames, 0, len(aliases))
	for alias := range aliases {
		sorted = append(sorted, alias)
	}
	sort.Sort(sorted)
	return sorted
}

type packageNames []tokens.PackageName

func (s packageNames) Len() int {
	return len(s)
}

func (s packageNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s packageNames) Less(i, j int) bool {
	return s[i] < s[j]
}

type moduleNames []tokens.ModuleName

func (s moduleNames) Len() int {
	return len(s)
}

func (s moduleNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s moduleNames) Less(i, j int) bool {
	return s[i] < s[j]
}

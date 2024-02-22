// Copyright 2024-2024, Pulumi Corporation.
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

import * as pulumi from "@pulumi/pulumi"; // @pulumi dependency is not included;
import * as isFinite from "is-finite"; // npm dependency
import * as relative from "./relative"; // local dependency

const dynamicProvider: pulumi.dynamic.ResourceProvider = {
  async create(inputs) {
    return {
      id: `dyn-${Math.ceil(Math.random() * 1000)}`, outs: { isFinite: isFinite(42), magic: relative.fun() }
    };
  }
}

export class MyDynamicProviderResource extends pulumi.dynamic.Resource {
  constructor(name: string, opts?: pulumi.CustomResourceOptions) {
    super(dynamicProvider, name, {}, opts);
  }
}

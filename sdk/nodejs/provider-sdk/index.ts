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

import * as provider from "@pulumi/pulumi/provider";
import { PObject, PType } from "./types";

// TODO: We could assert at compile time that these items.
export type ModuleName = string;
export type ItemName = string;
export type PackageName<T> = string;

type Properties = { [k: string]: PType };

export abstract class CustomResource {
}

abstract class ComponentResource {
  constructor(inputs: Properties) {
    console.log(`Inputs collected: ${new Schema<string>().v}`);
  }
}

type Module = { [k: ItemName]: Element };
type Package = { [k: ModuleName]: Module };

type Element = typeof CustomResource | typeof ComponentResource;

// A code first provider.
export class Provider<P> implements provider.Provider {
  readonly name: string;
  readonly version: string;
  private readonly pkg: Package;
  constructor(name: PackageName<P>, version: string, pkg: Package) {
    this.name = name;
    this.version = version;
    this.pkg = pkg;
  }
}

type OneOf<A extends string, B extends string> = `"oneof": [ ${A}, ${B} ]`;

class Schema<T extends PType> {
  v: any;
  x:
    | undefined
    | (T extends string ? "string"
      : T extends number ? "number"
      : OneOf<"string", "number">) = undefined;
  constructor(x: string) {
      this.v = (T extends string ? T : "") === x;
  }
}

class SomeComponent extends ComponentResource {
  constructor(inputs: { foo: string; bar: number }) {
    super(inputs);
  }
}

new Provider("fo:", "1.2.3", {
  "foobar": {
    foo: CustomResource,
  },
});

console.log(new Schema<string>().v);

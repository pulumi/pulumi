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

export type ID = string;
export type URN = string;

// Resource represents a class whose CRUD operations are implemented by a provider plugin.
export abstract class Resource {
    public readonly id: ID;   // the provider-assigned unique ID (initialized by the runtime).
    public readonly urn: URN; // the Lumi URN (initialized by the runtime).
}

// NamedResource is a kind of resource that has a friendly resource name associated with it.
export abstract class NamedResource extends Resource {
    public readonly name: string;

    constructor(name: string) {
        super();
        if (name === undefined || name === "") {
            throw new Error("Named resources must have a name");
        }
        this.name = name;
    }
}


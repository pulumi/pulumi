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

// Resource represents a class whose CRUD operations are implemented by a provider plugin.
export abstract class Resource {
    constructor() {
    }
}

// out indicates that a property is an output from the resource provider.  Such properties are treated differently by
// the runtime because their values come from outside of the Lumi type system.  Furthermore, the runtime permits
// speculative evaluation of code that depends upon them, in some circumstances, before the real value is known.
export function out(target: Object, propertyKey: string) {
    // nothing to do here; this is purely a decorative metadata token.
}


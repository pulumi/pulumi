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

// controller refers to the Fission controller endpoint URL in the target Kubernetes cluster.
export let controller: string | undefined;

// requireController fetches the Fission controller endpoint URL in the target Kubernetes cluster, requiring that it
// exists; if it has not been configured, an error is thrown.
export function requireController(): string {
    if (controller === undefined) {
        throw new Error("No Fission controller URL has been configured (`kube-fission:config:controller`)");
    }
    return controller;
}


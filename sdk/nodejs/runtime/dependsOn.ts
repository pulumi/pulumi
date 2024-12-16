// Copyright 2016-2024, Pulumi Corporation.
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

import { getAllResources, Input, Output } from "../output";
import { Resource } from "../resource";

/**
 * Gathers explicit dependent Resources from a list of Resources (possibly Promises and/or Outputs).
 *
 * @internal
 */
export async function gatherExplicitDependencies(
    dependsOn: Input<Array<Output<unknown> | Resource>> | Output<unknown> | Resource | undefined,
): Promise<Resource[]> {
    if (dependsOn) {
        if (Array.isArray(dependsOn)) {
            const dos: Resource[] = [];
            for (const d of dependsOn) {
                dos.push(...(await gatherExplicitDependencies(d)));
            }
            return dos;
        } else if (dependsOn instanceof Promise) {
            return gatherExplicitDependencies(await dependsOn);
        } else if (Output.isInstance(dependsOn)) {
            // Recursively gather dependencies, await the promise, and append the output's dependencies.
            const dos = (dependsOn as Output<Array<Output<unknown> | Resource> | unknown>).apply((v) => {
                if (Array.isArray(v)) {
                    return gatherExplicitDependencies(v);
                } else if (Resource.isInstance(v)) {
                    return gatherExplicitDependencies([v]);
                }
                return [] as Resource[];
            }
            );
            const urns = await dos.promise();
            const dosResources = await getAllResources(dos);
            const implicits = await gatherExplicitDependencies([...dosResources]);
            return (urns ?? []).concat(implicits);
        } else {
            // if (!Resource.isInstance(dependsOn)) {
            //     throw new Error("'dependsOn' was passed a value that was not a Resource.");
            // }

            return [dependsOn];
        }
    }

    return [];
}

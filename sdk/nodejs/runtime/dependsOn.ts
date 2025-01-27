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
import { ComponentResource, CustomResource, Resource } from "../resource";

/**
 * Gathers explicit dependent Resources from a list of Resources (possibly Promises and/or Outputs).
 *
 * @internal
 */
export async function gatherExplicitDependencies(
    dependsOn: Input<Input<Resource>[]> | Input<Resource> | undefined,
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
            const dos = (dependsOn as Output<Input<Resource>[] | Input<Resource>>).apply((v) =>
                gatherExplicitDependencies(v),
            );
            const urns = await dos.promise();
            const dosResources = await getAllResources(dos);
            const implicits = await gatherExplicitDependencies([...dosResources]);
            return (urns ?? []).concat(implicits);
        } else {
            if (!Resource.isInstance(dependsOn)) {
                throw new Error("'dependsOn' was passed a value that was not a Resource.");
            }

            return [dependsOn];
        }
    }

    return [];
}

/**
 * Go through 'resources', but transitively walk through **Component** resources, collecting any
 * of their child resources.  This way, a Component acts as an aggregation really of all the
 * reachable resources it parents.  This walking will stop when it hits custom resources.
 *
 * This function also terminates at remote components, whose children are not known to the Node SDK directly.
 * Remote components will always wait on all of their children, so ensuring we return the remote component
 * itself here and waiting on it will accomplish waiting on all of it's children regardless of whether they
 * are returned explicitly here.
 *
 * In other words, if we had:
 *
 *                  Comp1
 *              /     |     \
 *          Cust1   Comp2  Remote1
 *                  /   \       \
 *              Cust2   Cust3  Comp3
 *              /                 \
 *          Cust4                Cust5
 *
 * Then the transitively reachable resources of Comp1 will be [Cust1, Cust2, Cust3, Remote1].
 * It will *not* include:
 *   * Cust4 because it is a child of a custom resource
 *   * Comp2 because it is a non-remote component resource
 *   * Comp3 and Cust5 because Comp3 is a child of a remote component resource
 *
 * To do this, first we just get the transitively reachable set of resources (not diving
 * into custom resources).  In the above picture, if we start with 'Comp1', this will be
 * [Comp1, Cust1, Comp2, Cust2, Cust3]
 *
 * @internal
 */
export async function getAllTransitivelyReferencedResources(
    resources: Set<Resource>,
    exclude: Set<Resource>,
): Promise<Array<Resource>> {
    const transitivelyReachableResources = await getTransitivelyReferencedChildResourcesOfComponentResources(
        resources,
        exclude,
    );

    // Then we filter to only include Custom and Remote resources.
    const transitivelyReachableCustomResources = [...transitivelyReachableResources].filter(
        (r) => (CustomResource.isInstance(r) || (r as ComponentResource).__remote) && !exclude.has(r),
    );
    return transitivelyReachableCustomResources;
}

/**
 * Gather all URNs of resources transitively reachable from the given resources,
 * see `getAllTransitivelyReferencedResources`.
 *
 * @internal
 */
export async function getAllTransitivelyReferencedResourceURNs(
    resources: Set<Resource>,
    exclude: Set<Resource>,
): Promise<Set<string>> {
    const transitivelyReachableCustomResources = await getAllTransitivelyReferencedResources(resources, exclude);
    const promises = transitivelyReachableCustomResources.map((r) => r.urn.promise());
    const urns = await Promise.all(promises);
    return new Set<string>(urns);
}

/**
 * Recursively walk the resources passed in, returning them and all resources
 * reachable from {@link Resource.__childResources} through any **component**
 * resources we encounter.
 */
async function getTransitivelyReferencedChildResourcesOfComponentResources(
    resources: Set<Resource>,
    exclude: Set<Resource>,
) {
    // Recursively walk the dependent resources through their children, adding them to the result set.
    const result = new Set<Resource>();
    await addTransitivelyReferencedChildResourcesOfComponentResources(resources, exclude, result);
    return result;
}

async function addTransitivelyReferencedChildResourcesOfComponentResources(
    resources: Set<Resource> | undefined,
    exclude: Set<Resource>,
    result: Set<Resource>,
) {
    if (resources) {
        for (const resource of resources) {
            if (!result.has(resource)) {
                result.add(resource);

                if (ComponentResource.isInstance(resource)) {
                    // Skip including children of a resource in the excluded set to avoid depending on
                    // children that haven't been registered yet.
                    if (exclude.has(resource)) {
                        continue;
                    }

                    // This await is safe even if __isConstructed is undefined. Ensure that the
                    // resource has completely finished construction.  That way all parent/child
                    // relationships will have been setup.
                    await resource.__data;
                    const children = resource.__childResources;
                    addTransitivelyReferencedChildResourcesOfComponentResources(children, exclude, result);
                }
            }
        }
    }
}

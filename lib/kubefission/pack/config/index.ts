// Copyright 2017 Pulumi, Inc. All rights reserved.

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


import * as pulumi from "@pulumi/pulumi";

function singleOrNone<T>(elements: pulumi.Input<T>[]): pulumi.Input<T> | undefined {
    if (elements.length == 1) {
        return elements[0];
    }
    return undefined;
}

export const result = singleOrNone([1]);

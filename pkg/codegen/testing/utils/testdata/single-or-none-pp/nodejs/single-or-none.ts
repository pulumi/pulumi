import * as pulumi from "@pulumi/pulumi";

function singleOrNone<T>(elements: pulumi.Input<T>[]): pulumi.Input<T> {
    if (elements.length != 1) {
        throw new Error("singleOrNone expected input list to have a single element");
    }
    return elements[0];
}

export const result = singleOrNone([1]);

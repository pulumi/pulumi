import * as pulumi from "@pulumi/pulumi";

function singleOrNone<T>(elements: pulumi.Input<T>[]): pulumi.Input<T> | undefined {
    if (elements.length > 1) {
        throw new Error("singleOrNone expected input list to have a single element");
    }
    return elements[0];
}

const config = new pulumi.Config();
const aList = config.requireObject<Array<string>>("aList");
const singleOrNoneList = config.requireObject<Array<string>>("singleOrNoneList");
const aString = config.require("aString");
export const elementOutput1 = aList[1];
export const elementOutput2 = aList[2];
export const joinOutput = aList.join("|");
export const lengthOutput = aList.length;
export const splitOutput = aString.split("-");
export const singleOrNoneOutput = [singleOrNone(singleOrNoneList)];

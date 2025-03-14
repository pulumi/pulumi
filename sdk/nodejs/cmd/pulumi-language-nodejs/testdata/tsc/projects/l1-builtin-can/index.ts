import * as pulumi from "@pulumi/pulumi";

function canOutput_(
    fn: () => pulumi.Input<unknown>
): pulumi.Output<boolean> {
    try {
        // @ts-ignore
        return pulumi.output(fn()).apply(result => result !== undefined);
    } catch {
        return pulumi.output(false);
    }
}


function can_(
    fn: () => unknown
): boolean {
    try {
        const result = fn();
        if (result === undefined) {
            return false;
        }
        return true;
    } catch (e) {
        return false;
    }
}


const str = "str";
const aList = [
    "a",
    "b",
    "c",
];
export const nonOutputCan = // @ts-ignore
can_(() => aList[0]);
const config = new pulumi.Config();
const object = config.requireObject<any>("object");
const anotherObject = {
    nested: "nestedValue",
};
export const canFalse = // @ts-ignore
canOutput_(() => object.a);
export const canFalseDoubleNested = // @ts-ignore
canOutput_(() => object.a.b);
export const canTrue = // @ts-ignore
can_(() => anotherObject.nested);
// canOutput should also generate, secrets are l1 functions which return outputs.
const someSecret = pulumi.secret({
    a: "a",
});
export const canOutput = // @ts-ignore
canOutput_(() => someSecret.a).apply(can => can ? "true" : "false");

import * as pulumi from "@pulumi/pulumi";

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
const config = new pulumi.Config();
const object = config.requireObject("object");
const anotherObject = {
    nested: "nestedValue",
};
export const canFalse = // @ts-ignore
can_(() => object.a);
export const canFalseDoubleNested = // @ts-ignore
can_(() => object.a.b);
export const canTrue = // @ts-ignore
can_(() => anotherObject.nested);

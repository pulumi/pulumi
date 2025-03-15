import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

function canOutput_(
    fn: () => unknown
): pulumi.Output<boolean> {
    try {
        const result = fn();
        if (result === undefined) {
            return pulumi.output(false)
        }
        return pulumi.output(true)
    } catch (e) {
        return pulumi.output(false)
    }
}


function can_(
    fn: () => unknown
): boolean {
    try {
        const result = fn();
        if (result === undefined) {
            return false
        }
        return true
    } catch (e) {
        return false
    }
}


const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
export const canWithOutput = // @ts-ignore
canOutput_(() => component1.ref);
const str = "str";
export const canScalar = // @ts-ignore
can_(() => str);

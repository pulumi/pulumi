import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as simple_invoke from "@pulumi/simple-invoke";

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


const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
const componentCanShouldBeTrue = // @ts-ignore
canOutput_(() => component1.ref);
export const componentCan = componentCanShouldBeTrue;
const invokeCanShouldBeTrue = // @ts-ignore
canOutput_(() => simple_invoke.myInvokeOutput({
    value: "hello",
}));
export const invokeCan = invokeCanShouldBeTrue;
const ternaryShouldNotUseApply = // @ts-ignore
can_(() => true) ? "option_one" : "option_two";
export const ternaryCan = ternaryShouldNotUseApply;
const ternaryShouldUseApply = // @ts-ignore
canOutput_(() => component1.ref).apply(can => can ? "option_one" : "option_two");
export const ternaryCanOutput = ternaryShouldUseApply;
const str = "str";
export const scalarCan = // @ts-ignore
can_(() => str);

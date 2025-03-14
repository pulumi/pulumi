import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as simple_invoke from "@pulumi/simple-invoke";

function tryOutput_(
    ...fns: Array<() => unknown>
): pulumi.Output<any> {
    for (const fn of fns) {
        try {
            const result = fn();
            if (result === undefined) {
                continue;
            }
            return pulumi.output(result);
        } catch (e) {
            continue;
        }
    }
    throw new Error("try: all parameters failed");
}


function try_(
    ...fns: Array<() => unknown>
): any {
    for (const fn of fns) {
        try {
            const result = fn();
            if (result === undefined) {
                continue;
            }
            return result;
        } catch (e) {
            continue;
        }
    }
    throw new Error("try: all parameters failed");
}


const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
export const tryWithOutput = component1.ref.apply(ref => try_(
    // @ts-ignore
    () => ref,
    // @ts-ignore
    () => "failure"
));
const resultContainingOutput = simple_invoke.myInvokeOutput({
    value: "hello",
}).apply(invoke => try_(
    // @ts-ignore
    () => invoke
)).apply(_try => _try.result);
export const hello = resultContainingOutput;
const str = "str";
export const tryScalar = try_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => "fallback"
);

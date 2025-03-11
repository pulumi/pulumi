import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";

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
export const tryWithOutput = tryOutput_(
    // @ts-ignore
    () => component1.ref,
    // @ts-ignore
    () => "failure"
);
const str = "str";
export const tryScalar = try_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => "fallback"
);

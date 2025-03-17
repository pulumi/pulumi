import * as pulumi from "@pulumi/pulumi";

function tryOutput_(
    ...fns: Array<() => pulumi.Input<unknown>>
): pulumi.Output<any> {
    if (fns.length === 0) {
	       throw new Error("try: all parameters failed");
    }

    const [fn, ...rest] = fns;
    let resultOutput: pulumi.Output<any> | undefined;
    try {
        const result = fn();
        if (result === undefined) {
            return tryOutput_(...rest);
        }
        resultOutput = pulumi.output(result);
    } catch {
	       return tryOutput_(...rest);
    }

    // @ts-ignore
	   return resultOutput;
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


const str = "str";
const aList = [
    "a",
    "b",
    "c",
];
export const nonOutputTry = try_(
    // @ts-ignore
    () => aList[0],
    // @ts-ignore
    () => "fallback"
);
const config = new pulumi.Config();
const object = config.requireObject("object");
export const trySucceed = tryOutput_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => "fallback"
);
export const tryFallback1 = tryOutput_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => "fallback"
);
export const tryFallback2 = tryOutput_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => object.b,
    // @ts-ignore
    () => "fallback"
);
export const tryMultipleTypes = tryOutput_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => object.b,
    // @ts-ignore
    () => 42,
    // @ts-ignore
    () => "fallback"
);

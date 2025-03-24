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


const config = new pulumi.Config();
const aMap = config.requireObject<Record<string, string>>("aMap");
export const plainCanSuccess = // @ts-ignore
can_(() => aMap.a);
export const plainCanFailure = // @ts-ignore
can_(() => aMap.b);
const aSecretMap = pulumi.secret(aMap);
export const outputCanSuccess = // @ts-ignore
canOutput_(() => aSecretMap.a);
export const outputCanFailure = // @ts-ignore
canOutput_(() => aSecretMap.b);
const anObject = config.requireObject<any>("anObject");
export const dynamicCanSuccess = // @ts-ignore
canOutput_(() => anObject.a);
export const dynamicCanFailure = // @ts-ignore
canOutput_(() => anObject.b);
const aSecretObject = pulumi.secret(anObject);
export const outputDynamicCanSuccess = // @ts-ignore
canOutput_(() => aSecretObject.apply(aSecretObject => aSecretObject.a));
export const outputDynamicCanFailure = // @ts-ignore
canOutput_(() => aSecretObject.apply(aSecretObject => aSecretObject.b));

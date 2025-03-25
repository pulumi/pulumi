import * as pulumi from "@pulumi/pulumi";

function canOutput_(
    fn: () => pulumi.Input<any>
): pulumi.Output<boolean> {
    try {
        return pulumi.output(fn()).apply(result => result !== undefined);
    } catch {
        return pulumi.output(false);
    }
}


function can_(
    fn: () => any
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
export const plainTrySuccess = 
can_(() => aMap.a);
export const plainTryFailure = 
can_(() => aMap.b);
const aSecretMap = pulumi.secret(aMap);
export const outputTrySuccess = 
canOutput_(() => aSecretMap.a);
export const outputTryFailure = 
canOutput_(() => aSecretMap.b);
const anObject = config.requireObject<any>("anObject");
export const dynamicTrySuccess = 
canOutput_(() => anObject.a);
export const dynamicTryFailure = 
canOutput_(() => anObject.b);
const aSecretObject = pulumi.secret(anObject);
export const outputDynamicTrySuccess = 
canOutput_(() => aSecretObject.apply(aSecretObject => aSecretObject.a));
export const outputDynamicTryFailure = 
canOutput_(() => aSecretObject.apply(aSecretObject => aSecretObject.b));
export const plainTryNull = 
canOutput_(() => anObject.opt);
export const outputTryNull = 
canOutput_(() => aSecretObject.apply(aSecretObject => aSecretObject.opt));

import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";

export = async () => {
    const provElided = new output.Provider("provElided", {elideUnknowns: true});
    const provNotElided = new output.Provider("provNotElided", {});
    const topLevelElided = new output.Resource("topLevelElided", {value: 1}, {
        provider: provElided,
    });
    const topLevelNotElided = new output.Resource("topLevelNotElided", {value: 1}, {
        provider: provNotElided,
    });
    return {
        topLevelElided: topLevelElided.secretOutput,
        topLevelNotElided: topLevelNotElided.secretOutput,
    };
}

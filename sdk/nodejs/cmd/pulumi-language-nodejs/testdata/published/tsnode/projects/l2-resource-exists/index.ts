import * as pulumi from "@pulumi/pulumi";
import * as exists from "@pulumi/exists";

export = async () => {
    const res = new exists.Resource("res", {value: true});
    const existsResult = pulumi.runtime.existsResource("exists:index:Resource", res.id, {}, {});
    const existsWithState = pulumi.runtime.existsResource("exists:index:Resource", res.id, {
        value: true,
    }, {});
    const missingWithState = pulumi.runtime.existsResource("exists:index:Resource", res.id, {
        value: false,
    }, {});
    return {
        existsResult: existsResult,
        existsWithState: existsWithState,
        missingWithState: missingWithState,
    };
}

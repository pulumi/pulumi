import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

export default async () => {
    const res = new simple.Resource("res", {value: true});
    const existsResult = pulumi.runtime.existsResource("simple:index:Resource", "checkExists", res.id);
    return {
        existsResult: existsResult,
    };
}

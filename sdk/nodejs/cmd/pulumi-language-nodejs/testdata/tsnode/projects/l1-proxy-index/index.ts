import * as pulumi from "@pulumi/pulumi";

export = async () => {
    const config = new pulumi.Config();
    const object = config.requireObject<{property?: string}>("object");
    const l = pulumi.secret([1]);
    const m = pulumi.secret({
        key: true,
    });
    const c = pulumi.secret(object);
    const o = pulumi.secret({
        property: "value",
    });
    return {
        l: l[0],
        m: m.key,
        c: c.apply(c => c.property),
        o: o.property,
    };
}

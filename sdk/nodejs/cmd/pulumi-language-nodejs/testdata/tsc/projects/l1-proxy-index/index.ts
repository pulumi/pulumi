import * as pulumi from "@pulumi/pulumi";

export = async () => {
    const config = new pulumi.Config();
    const anObject = config.requireObject<{property?: string}>("anObject");
    const anyObject = config.requireObject<any>("anyObject");
    const l = pulumi.secret([1]);
    const m = pulumi.secret({
        key: true,
    });
    const c = pulumi.secret(anObject);
    const o = pulumi.secret({
        property: "value",
    });
    const a = pulumi.secret(anyObject);
    return {
        l: l[0],
        m: m.key,
        c: c.apply(c => c.property),
        o: o.property,
        a: a.apply(a => a.property),
    };
}

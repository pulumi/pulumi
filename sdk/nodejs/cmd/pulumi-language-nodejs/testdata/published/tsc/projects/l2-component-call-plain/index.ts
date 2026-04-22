import * as pulumi from "@pulumi/pulumi";
import * as configurer from "@pulumi/configurer";

export = async () => {
    const configurer2 = new configurer.Configurer("configurer", {providerConfig: "propagated"});
    const customFromPlainProvider = new configurer.Custom("customFromPlainProvider", {value: "from-plain-provider"}, {
        provider: (await configurer2.plainProvider()),
    });
    const customFromNestedPlainProvider = new configurer.Custom("customFromNestedPlainProvider", {value: "from-nested-plain-provider"}, {
        provider: (await configurer2.nestedPlainProvider()).provider,
    });
    return {
        plainValue: (await configurer2.plainValue()),
        nestedPlainValue: (await configurer2.nestedPlainProvider()).value,
    };
}

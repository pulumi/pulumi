import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

export default async () => {
    const config = new pulumi.Config();
    const configLexicalName = config.requireBoolean("cC-Charlie_charlie.😃⁉️");
    const resourceLexicalName = new simple.Resource("aA-Alpha_alpha.🤯⁉️", {value: configLexicalName});
    return {
        "bB-Beta_beta.💜⁉": resourceLexicalName.value,
        "dD-Delta_delta.🔥⁉": resourceLexicalName.value,
    };
}

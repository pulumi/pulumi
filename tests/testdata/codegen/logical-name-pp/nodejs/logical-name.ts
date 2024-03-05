import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

export = async () => {
    const config = new pulumi.Config();
    const configLexicalName = config.require("cC-Charlie_charlie.😃⁉️");
    const resourceLexicalName = new random.RandomPet("aA-Alpha_alpha.🤯⁉️", {prefix: configLexicalName});
    return {
        "bB-Beta_beta.💜⁉": resourceLexicalName.id,
        "dD-Delta_delta.🔥⁉": resourceLexicalName.id,
    };
}

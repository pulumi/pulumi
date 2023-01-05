import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

export = async () => {
    const config = new pulumi.Config();
    const configLexicalName = config.require("cC-Charlie_charlie.😃⁉️");
    const resourceLexicalName = new random.RandomPet("aA-Alpha_alpha.🤯⁉️", {prefix: configLexicalName});
    const outputLexicalName = resourceLexicalName.id;
    return {
        "bB-Beta_beta.💜⁉": outputLexicalName,
    };
}

import * as pulumi from "@pulumi/pulumi";
import * as random from "@pulumi/random";

export = async () => {
    const resourceLexicalName = new random.RandomPet("aA-Alpha_alpha.🤯⁉️", {});
    const outputLexicalName = resourceLexicalName.id;
    return {
        "bB-Beta_beta.💜⁉": outputLexicalName,
    };
}

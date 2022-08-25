import * as pulumi from "@pulumi/pulumi";
import * as other from "@third-party/other";

const Other = new other.Thing("Other", {idea: "Support Third Party"});
const Question = new other.module.Object("Question", {answer: 42});
const Provider = new other.Provider("Provider", {});

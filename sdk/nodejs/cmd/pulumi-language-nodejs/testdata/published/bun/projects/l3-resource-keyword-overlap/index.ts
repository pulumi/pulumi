import * as pulumi from "@pulumi/pulumi";
import { KeywordComponent } from "./keywordComponent";

const comp = new KeywordComponent("comp", {input: true});
export const result = comp.result;

import * as pulumi from "@pulumi/pulumi";
import { First } from "./first";
import { Second } from "./second";

const first = new First("first", {passwordLength: second.passwordLength});
const second = new Second("second", {petName: first.petName});

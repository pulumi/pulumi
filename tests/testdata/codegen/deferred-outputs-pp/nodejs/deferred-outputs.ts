import * as pulumi from "@pulumi/pulumi";
import { First } from "./first";
import { Second } from "./second";

const [secondPasswordLength, resolveSecondPasswordLength] = pulumi.deferredOutput();
const first = new First("first", {passwordLength: secondPasswordLength});
const second = new Second("second", {petName: first.petName});
resolveSecondPasswordLength(second.passwordLength);

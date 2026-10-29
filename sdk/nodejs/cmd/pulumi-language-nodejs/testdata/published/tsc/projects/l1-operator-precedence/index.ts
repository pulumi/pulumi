import * as pulumi from "@pulumi/pulumi";

export const sumTimes = (1 + 2) * 3;
export const minusMinus = 10 - (4 - 3);
export const notAnd = !(true && false);
export const negateSum = -(1 + 2);
export const conditionalIndex = (true ? [1] : [2])[0];
export const conditionalCondition = (true ? false : true) ? "yes" : "no";

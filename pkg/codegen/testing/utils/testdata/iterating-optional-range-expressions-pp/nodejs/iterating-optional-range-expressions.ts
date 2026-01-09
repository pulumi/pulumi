import * as pulumi from "@pulumi/pulumi";
import * as range from "@pulumi/range";

const root = new range.Root("root", {});
// creating resources by iterating a property of type array(string) of another resource
const fromListOfStrings: range.Example[] = [];
root.arrayOfString.apply(rangeBody => {
    for (const range of rangeBody.map((v, k) => ({key: k, value: v}))) {
        fromListOfStrings.push(new range.Example(`fromListOfStrings-${range.key}`, {someString: range.value}));
    }
});
// creating resources by iterating a property of type map(string) of another resource
const fromMapOfStrings: range.Example[] = [];
root.mapOfString.apply(rangeBody => {
    for (const range of Object.entries(rangeBody).map(([k, v]) => ({key: k, value: v}))) {
        fromMapOfStrings.push(new range.Example(`fromMapOfStrings-${range.key}`, {someString: `${range.key} ${range.value}`}));
    }
});
// computed range list expression to create instances of range:index:Example resource
const fromComputedListOfStrings: range.Example[] = [];
pulumi.all([
    root.mapOfString.apply(mapOfString => mapOfString?.hello),
    root.mapOfString.apply(mapOfString => mapOfString?.world),
]).apply(rangeBody => {
    for (const range of rangeBody.map((v, k) => ({key: k, value: v}))) {
        fromComputedListOfStrings.push(new range.Example(`fromComputedListOfStrings-${range.key}`, {someString: `${range.key} ${range.value}`}));
    }
});
// computed range for expression to create instances of range:index:Example resource
const fromComputedForExpression: range.Example[] = [];
pulumi.all([root.arrayOfString, root.mapOfString]).apply(([arrayOfString, mapOfString]) => {
    for (const range of arrayOfString.map(value => (mapOfString[value])).map((v, k) => ({key: k, value: v}))) {
        fromComputedForExpression.push(new range.Example(`fromComputedForExpression-${range.key}`, {someString: `${range.key} ${range.value}`}));
    }
});

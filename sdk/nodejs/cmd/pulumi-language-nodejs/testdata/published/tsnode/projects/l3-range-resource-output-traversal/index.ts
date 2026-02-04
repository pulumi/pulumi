import * as pulumi from "@pulumi/pulumi";
import * as dns from "@pulumi/dns";

const subscription = new dns.Subscription("subscription", {domains: [
    "example.com",
    "test.com",
]});
const record: dns.Record[] = [];
subscription.challenges.apply(rangeBody => {
    for (const range of rangeBody.map((v, k) => ({key: k, value: v}))) {
        record.push(new dns.Record(`record-${range.key}`, {name: range.value.recordName}));
    }
});

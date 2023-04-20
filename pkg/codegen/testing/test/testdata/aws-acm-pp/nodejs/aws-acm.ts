import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const plainDomain = "example.com";
const albCertificateArn = "someArn";
const zoneId = "eu";
const acm: aws.acm.Certificate[] = [];
for (const range = {value: 0}; range.value < (albCertificateArn == "" ? 1 : 0); range.value++) {
    acm.push(new aws.acm.Certificate(`acm-${range.value}`, {
        domainName: plainDomain,
        validationMethod: "DNS",
    }));
}
const dnsAcmValidation: aws.route53.Record[] = [];
for (const range of (albCertificateArn == "" ? acm[0].domainValidationOptions.apply(domainValidationOptions => domainValidationOptions.map(dvo => ({
    name: dvo.resourceRecordName,
    record: dvo.resourceRecordValue,
    type: dvo.resourceRecordType,
}))) : []).map((v, k) => ({key: k, value: v}))) {
    dnsAcmValidation.push(new aws.route53.Record(`dnsAcmValidation-${range.key}`, {
        name: range.value.name,
        type: aws.route53.recordtype.RecordType[range.value.type],
        zoneId: zoneId,
        records: [range.value.record],
        ttl: 60,
    }));
}
const acmValidation: aws.acm.CertificateValidation[] = [];
for (const range = {value: 0}; range.value < (albCertificateArn == "" ? 1 : 0); range.value++) {
    acmValidation.push(new aws.acm.CertificateValidation(`acmValidation-${range.value}`, {
        certificateArn: acm[0].arn,
        validationRecordFqdns: dnsAcmValidation.apply(dnsAcmValidation => dnsAcmValidation.map(record => (record.fqdn))),
    }));
}

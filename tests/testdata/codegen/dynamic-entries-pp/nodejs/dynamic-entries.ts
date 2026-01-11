import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
// A domain name for which the certificate should be issued
const domainName = config.get("domainName") || "";
// Which method to use for validation. DNS or EMAIL are valid, NONE can be used for certificates that were imported into ACM and then into Terraform.
const validationMethod = config.get("validationMethod") || "DNS";
const alternativeNames = config.getObject<any>("alternativeNames") || {};
const certificate = new aws.acm.Certificate("certificate", {
    subjectAlternativeNames: Object.entries(alternativeNames).map(([k, v]) => ({key: k, value: v})).map(entry => (entry.value)),
    domainName: domainName,
    validationMethod: validationMethod,
});

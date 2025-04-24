import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
// A domain name for which the certificate should be issued
const domainName = config.get("domainName") || "";
// Which method to use for validation. DNS or EMAIL are valid, NONE can be used for certificates that were imported into ACM and then into Terraform.
const validationMethod = config.get("validationMethod") || "DNS";
const validationOption = config.getObject<any>("validationOption") || {};
const certificate = new aws.acm.Certificate("certificate", {
    validationOptions: Object.entries(validationOption).map(([k, v]) => ({key: k, value: v})).map(entry => ({
        domainName: entry.value.domain_name,
        validationDomain: entry.value.validation_domain,
    })),
    domainName: domainName,
    validationMethod: validationMethod,
});

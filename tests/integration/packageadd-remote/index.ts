import * as tls from "@pulumi/tls-self-signed-cert";

const cert = new tls.SelfSignedCertificate("mycert", {
    subject: {
        organization: "Example Org"
    },
    dnsName: "example.com",
    validityPeriodHours: 24,
    localValidityPeriodHours: 24,
});
export const certPem = cert.pem;

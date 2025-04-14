import * as tls from "@pulumi/tls-self-signed-cert";

const cert = new tls.SelfSignedCertificate("mycert", {
    algorithm: tls.Algorithm.Rsa,
    subject: {
        organization: "Example Org"
    },
    dnsName: "example.com",
    ecdsaCurve: tls.EcdsaCurve.P256,
    validityPeriodHours: 24,
    localValidityPeriodHours: 24,
});
export const certPem = cert.pem;

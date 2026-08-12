import * as tlsSelfSignedCert from "tls-self-signed-cert";
import * as testProvider2 from "test-provider-2";

// Create a certificate using the first component (test-provider)
const cert1 = new tlsSelfSignedCert.SelfSignedCertificate("cert1", {
    subject: {
        organization: "Example Org 1"
    },
    dnsName: "example1.com",
    validityPeriodHours: 24,
    localValidityPeriodHours: 24,
});

// Create a certificate using the second component (test-provider-2)
const cert2 = new testProvider2.SelfSignedCertificate("cert2", {
    subject: {
        organization: "Example Org 2"
    },
    dnsName: "example2.com",
    validityPeriodHours: 24,
    localValidityPeriodHours: 24,
    algorithm: "RSA",
    rsaBits: 2048,
    ecdsaCurve: "P224",
});

// Export outputs from both components to verify both resources were created
export const cert1Pem = cert1.pem;
export const cert2Pem = cert2.pem;

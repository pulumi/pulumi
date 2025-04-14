// Copyright 2016-2024, Pulumi Corporation.

import * as pulumi from "@pulumi/pulumi";
import * as tls from "@pulumi/tls";

// This resources helps you create a self signed certificate.
export class SelfSignedCertificate extends pulumi.ComponentResource {
    // The PEM of your self signed cert.
    public readonly pem: pulumi.Output<string>;

    // The private key of your self signed cert.
    public readonly privateKey: pulumi.Output<string>;

    // Your self signed cert.
    public readonly caCert: pulumi.Output<string>;

    constructor(name: string, args: SelfSignedCertificateArgs, opts?: pulumi.ComponentResourceOptions) {
        super("tls-self-signed-cert:index:SelfSignedCertificate", name, args, opts);

        const algorithm = args.algorithm || "RSA";
        const rsaBits = args.rsaBits || 2048;
        const ecdsaCurve = args.ecdsaCurve || "P224";
        const allowedUses = args.allowedUses || [ "key_encipherment",  "digital_signature" ];

        if (!args.dnsName && !args.ipAddress) {
            throw new Error("At least one of `dnsName` or `ipAddress` must be provided as an input.");
        }

        // create a CA private key
        const caKey = new tls.PrivateKey(`${name}-ca`, {
            algorithm,
            ecdsaCurve,
            rsaBits,
        }, { parent: this });

        // create a CA certificate
        const caCert = new tls.SelfSignedCert(`${name}-ca`, {
            privateKeyPem: caKey.privateKeyPem,
            isCaCertificate: true,
            validityPeriodHours: args.validityPeriodHours,
            subject: args.subject,
            allowedUses,
        }, { parent: caKey });

        // Create a certificate private key
        const key = new tls.PrivateKey(`${name}-privateKey`, {
            algorithm,
            ecdsaCurve,
            rsaBits,
        }, { parent: caKey });

        const certRequest = new tls.CertRequest("certRequest", {
            privateKeyPem: key.privateKeyPem,
            dnsNames: args.dnsName ? [ args.dnsName ]: [],
            ipAddresses: args.ipAddress ? [ args.ipAddress ] : [],
            subject: {
                ...args.subject,
                commonName: args.dnsName,
            },
        }, { parent: key });

        const cert = new tls.LocallySignedCert("cert", {
            certRequestPem: certRequest.certRequestPem,
            caPrivateKeyPem: caKey.privateKeyPem,
            caCertPem: caCert.certPem,
            validityPeriodHours: args.localValidityPeriodHours,
            allowedUses,
        }, { parent: certRequest });

        this.pem = cert.certPem;
        this.privateKey = key.privateKeyPem;
        this.caCert = cert.caCertPem;
    }
}

export interface SelfSignedCertificateArgs {
    // Name of the algorithm to use when generating the private key. Currently-supported values are `RSA`, `ECDSA` and `ED25519` (default: `RSA`).
    algorithm?: pulumi.Input<Algorithm>;

    // When `algorithm` is `ECDSA`, the name of the elliptic curve to use. Currently-supported values are `P224`, `P256`, `P384` or `P521` (default: `P224`).
    ecdsaCurve?: pulumi.Input<EcdsaCurve>;

    // List of key usages allowed for the issued certificate. Values are defined in [RFC 5280](https://datatracker.ietf.org/doc/html/rfc5280) and combine flags defined by both [Key Usages](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.3) and [Extended Key Usages](https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.12). Accepted values: `any_extended`, `cert_signing`, `client_auth`, `code_signing`, `content_commitment`, `crl_signing`, `data_encipherment`, `decipher_only`, `digital_signature`, `email_protection`, `encipher_only`, `ipsec_end_system`, `ipsec_tunnel`, `ipsec_user`, `key_agreement`, `key_encipherment`, `microsoft_commercial_code_signing`, `microsoft_kernel_code_signing`, `microsoft_server_gated_crypto`, `netscape_server_gated_crypto`, `ocsp_signing`, `server_auth`, `timestamping`.
    allowedUses?: pulumi.Input<AllowedUses[]>;

    // When `algorithm` is `RSA`, the size of the generated RSA key, in bits (default: `2048`).
    rsaBits?: pulumi.Input<number>;

    // Number of hours, after initial issuing, that the certificate will remain valid for.
    validityPeriodHours: pulumi.Input<number>;

    // Number of hours, after initial issuing, that the local certificate will remain valid for.
    localValidityPeriodHours: pulumi.Input<number>;

    // The subject for which a certificate is being requested. The acceptable arguments are all optional and their naming is based upon [Issuer Distinguished Names (RFC5280)](https://tools.ietf.org/html/rfc5280#section-4.1.2.4) section.
    subject: SelfSignedCertSubject;

    // The DNS name for which a certificate is being requested (i.e. certificate subjects).
    dnsName?: pulumi.Input<string>;

    // The IP address for which a certificate is being requested (i.e. certificate subjects).
    ipAddress?: pulumi.Input<string>;
}

enum Algorithm {
    Rsa = "RSA",
    Ecdsa = "ECDSA",
    Ed25519 = "ED25519",
}

enum EcdsaCurve {
    P224 = "P224",
    P256 = "P256",
    P384 = "P384",
    P521 = "P521",
}

enum AllowedUses {
    AnyExtended = "any_extended",
    CertSigning = "cert_signing",
    ClientAuth = "client_auth",
    CodeSigning = "code_signing",
    ContentCommitment = "content_commitment",
    CrlSigning = "crl_signing",
    DataEncipherment = "data_encipherment",
    DecipherOnly = "decipher_only",
    DigitalSignature = "digital_signature",
    EmailProtection = "email_protection",
    EncipherOnly = "encipher_only",
    IpsecEndSystem = "ipsec_end_system",
    IpsecTunnel = "ipsec_tunnel",
    IpsecUser = "ipsec_user",
    KeyAgreement = "key_agreement",
    KeyEncipherment = "key_encipherment",
    MicrosoftCommercialCodeSigning = "microsoft_commercial_code_signing",
    MicrosoftKernelCodeSigning = "microsoft_kernel_code_signing",
    OcspSigning = "ocsp_signing",
    ServerAuth = "server_auth",
    Timestamping = "timestamping",
}

export interface SelfSignedCertSubject {
    /**
     * Distinguished name: `CN`
     */
    commonName?: pulumi.Input<string>;
    /**
     * Distinguished name: `C`
     */
    country?: pulumi.Input<string>;
    /**
     * Distinguished name: `L`
     */
    locality?: pulumi.Input<string>;
    /**
     * Distinguished name: `O`
     */
    organization?: pulumi.Input<string>;
    /**
     * Distinguished name: `OU`
     */
    organizationalUnit?: pulumi.Input<string>;
    /**
     * Distinguished name: `PC`
     */
    postalCode?: pulumi.Input<string>;
    /**
     * Distinguished name: `ST`
     */
    province?: pulumi.Input<string>;
    /**
     * Distinguished name: `SERIALNUMBER`
     */
    serialNumber?: pulumi.Input<string>;
    /**
     * Distinguished name: `STREET`
     */
    streetAddresses?: pulumi.Input<pulumi.Input<string>[]>;
}

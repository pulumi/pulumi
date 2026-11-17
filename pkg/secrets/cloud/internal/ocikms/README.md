# Internal OCI Key Management driver

This package adapts the official OCI Go SDK to the existing Go Cloud Secrets
Keeper contract used by Pulumi. It is internal to the cloud secrets manager;
there is no additional module or resource plugin to install.

The implementation adopts the corrected MIT-licensed driver from
[frankli0324/gocloud-ext](https://github.com/frankli0324/gocloud-ext), with
fixes contributed at revision
`88392e67caca6d65d29c8b0ac2646d1b2d3bd88a`. The original MIT notice is
preserved in LICENSE and included in distribution archives.

Use `ocikms://<crypto-endpoint>/<key-ocid>` as the stack secrets provider.
The existing Pulumi cloud manager owns envelope encryption and checkpoint
serialization. This adapter handles binary key encryption/decryption,
OCI authentication and native error exposure/classification.

`OCI_CLI_AUTH=api_key` selects one configuration file/profile through
`OCI_CLI_CONFIG_FILE` and `OCI_CLI_PROFILE`; `instance_principal` selects
OCI instance identity. Authentication failures do not select another identity.
Keep credentials out of URLs and logs.

From the repository root, run the offline package tests with:

```sh
go -C pkg test -count=1 ./secrets/cloud/internal/ocikms
```

For real OCI conformance, provide two distinct authorized key references in
`OCIKMS_TEST_URL` and `OCIKMS_SECOND_TEST_URL`, then run:

```sh
go -C pkg test -count=1 -tags ociintegration -run '^TestOCI' ./secrets/cloud/internal/ocikms ./secrets/cloud
```

The selected integration tests fail when references are absent. They do not
replace OCI with a simulated service or skip missing credentials.

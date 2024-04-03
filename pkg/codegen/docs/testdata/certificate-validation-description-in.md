This resource is a test case for rendering legacy examples shortcodes correctly.
It should not render any of the below examples in docInfo.description; only this description.

{{% examples %}}
## Example Usage

{{% example %}}
### DNS Validation with Route 53

```typescript
import * as aws from \"@pulumi/aws\";

const exampleCertificate = new aws.acm.Certificate(\"exampleCertificate\", {
 domainName: \"example.com\",
 validationMethod: \"DNS\",
});
const exampleZone = aws.route53.getZone({
 name: \"example.com\",
 privateZone: false,
});

const certValidation = new aws.route53.Record(\"certValidation\", {
 name: exampleCertificate.domainValidationOptions[0].resourceRecordName,
 records: [exampleCertificate.domainValidationOptions[0].resourceRecordValue],
 ttl: 60,
 type: exampleCertificate.domainValidationOptions[0].resourceRecordType,
 zoneId: exampleZone.then(x => x.zoneId),
});

const certCertificateValidation = new aws.acm.CertificateValidation(\"cert\", {
 certificateArn: exampleCertificate.arn,
 validationRecordFqdns: [certValidation.fqdn],
});

export const certificateArn = certCertificateValidation.certificateArn;
```
```go
package main

import (
	\"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/acm\"
	\"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/route53\"
	\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
 exampleCertificate, err := acm.NewCertificate(ctx, \"exampleCertificate\", &acm.CertificateArgs{
 DomainName: pulumi.String(\"example.com\"),
 ValidationMethod: pulumi.String(\"DNS\"),
 })
 if err != nil {
 return err
 }
 
 exampleZone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
 Name: pulumi.StringRef(\"example.com\"),
 PrivateZone: pulumi.BoolRef(false),
 }, nil)
 if err != nil {
 return err
 }
 
 domainValidationOption := exampleCertificate.DomainValidationOptions.ApplyT(func(options []acm.CertificateDomainValidationOption) interface{} {
 return options[0]
 })
 
 certValidation, err := route53.NewRecord(ctx, \"certValidation\", &route53.RecordArgs{
 Name: domainValidationOption.ApplyT(func(option interface{}) string {
 return *option.(acm.CertificateDomainValidationOption).ResourceRecordName
 }).(pulumi.StringOutput),
 Type: domainValidationOption.ApplyT(func(option interface{}) string {
 return *option.(acm.CertificateDomainValidationOption).ResourceRecordType
 }).(pulumi.StringOutput),
 Records: pulumi.StringArray{
 domainValidationOption.ApplyT(func(option interface{}) string {
 return *option.(acm.CertificateDomainValidationOption).ResourceRecordValue
 }).(pulumi.StringOutput),
 },
 Ttl: pulumi.Int(10 * 60),
 ZoneId: pulumi.String(exampleZone.ZoneId),
 })
 if err != nil {
 return err
 }
 
 certCertificateValidation, err := acm.NewCertificateValidation(ctx, \"cert\", &acm.CertificateValidationArgs{
 CertificateArn: exampleCertificate.Arn,
 ValidationRecordFqdns: pulumi.StringArray{
 certValidation.Fqdn,
 },
 })
 if err != nil {
 return err
 }
 
 ctx.Export(\"certificateArn\", certCertificateValidation.CertificateArn)
 
 return nil
 })
}
```
```python
import pulumi_aws as aws

example_certificate = aws.acm.Certificate(\"exampleCertificate\",
 domain_name=\"example.com\",
 validation_method=\"DNS\")

example_zone = aws.route53.getZone(name=\"example.com\",
 private_zone=False)

cert_validation = aws.route53.Record(\"certValidation\",
 name=example_certificate.domain_validation_options[0].resource_record_name,
 records=[example_certificate.domain_validation_options[0].resource_record_value],
 ttl=60,
 type=example_certificate.domain_validation_options[0].resource_record_type,
 zone_id=example_zone.zone_id)

cert_certificate_validation = aws.acm.CertificateValidation(\"cert\",
 certificate_arn=example_certificate.arn,
 validation_record_fqdns=[cert_validation.fqdn])

pulumi.export(\"certificate_arn\", cert_certificate_validation.certificate_arn)
```
```csharp
using Pulumi;
using Pulumi.Aws.Acm;
using Pulumi.Aws.Route53;
using System.Collections.Generic;

return await Deployment.RunAsync(() =>
{
 var exampleCertificate = new Certificate(\"exampleCertificate\", new CertificateArgs
 {
 DomainName = \"example.com\",
 ValidationMethod = \"DNS\"
 });

 var exampleZone = GetZone.Invoke(new GetZoneInvokeArgs
 {
 Name = \"example.com\",
 PrivateZone = false,
 });

 var certValidation = new Record(\"certValidation\", new RecordArgs
 {
 Name = exampleCertificate.DomainValidationOptions.Apply(options => options[0].ResourceRecordName!),
 Records =
 {
 exampleCertificate.DomainValidationOptions.Apply(options => options[0].ResourceRecordValue!),
 },
 Ttl = 60,
 Type = exampleCertificate.DomainValidationOptions.Apply(options => options[0].ResourceRecordType!),
 ZoneId = exampleZone.Apply(zone => zone.Id),
 });

 var certCertificateValidation = new CertificateValidation(\"cert\", new CertificateValidationArgs
 {
 CertificateArn = exampleCertificate.Arn,
 ValidationRecordFqdns =
 {
 certValidation.Fqdn,
 },
 });
 
 return new Dictionary<string, object?>
 {
 [\"certificateArn\"] = certCertificateValidation.CertificateArn,
 };
});
```
```yaml
variables:
 zoneId:
 Fn::Invoke:
 Function: aws.route53.getZone
 Arguments:
 name: \"example.com\"
 privateZone: false
 Return: id
resources:
 exampleCertificate:
 type: aws.acm.Certificate
 properties:
 domainName: \"example.com\"
 validationMethod: \"DNS\"
 certValidation:
 type: aws.route53.Record
 properties:
 name: ${exampleCertificate.domainValidationOptions[0].resourceRecordName}
 records: [${exampleCertificate.domainValidationOptions[0].resourceRecordValue}]
 ttl: 60
 type: ${exampleCertificate.domainValidationOptions[0].resourceRecordType}
 zoneId: ${zoneId}
 certCertificateValidation:
 type: aws.acm.CertificateValidation
 properties:
 certificateArn: ${exampleCertificate.arn}
 validationRecordFqdns: [${certValidation.fqdn}]
outputs:
 certificateArn: ${certCertificateValidation.certificateArn}
```
{{% /example %}}
{{% example %}}
### Email Validation

```typescript
import * as aws from \"@pulumi/aws\";

const exampleCertificate = new aws.acm.Certificate(\"exampleCertificate\", {
 domainName: \"example.com\",
 validationMethod: \"EMAIL\",
});

const exampleCertificateValidation = new aws.acm.CertificateValidation(\"exampleCertificateValidation\", {
 certificateArn: exampleCertificate.arn,
});
```
```go
package main

import (
	\"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/acm\"
	\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
 exampleCertificate, err := acm.NewCertificate(ctx, \"exampleCertificate\", &acm.CertificateArgs{
 DomainName: pulumi.String(\"example.com\"),
 ValidationMethod: pulumi.String(\"EMAIL\"),
 })
 if err != nil {
 return err
 }
 
 _, err = acm.NewCertificateValidation(ctx, \"exampleCertificateValidation\", &acm.CertificateValidationArgs{
 CertificateArn: exampleCertificate.Arn,
 })
 if err != nil {
 return err
 }
		return nil
	})
}
```
```python
import pulumi_aws as aws

example_certificate = aws.acm.Certificate(\"exampleCertificate\",
 domain_name=\"example.com\",
 validation_method=\"EMAIL\")

example_certificate_validation = aws.acm.CertificateValidation(\"exampleCertificateValidation\",
 certificate_arn=example_certificate.arn)
```
```csharp
using Pulumi;
using Pulumi.Aws.Acm;

return await Deployment.RunAsync(() =>
{
 var exampleCertificate = new Certificate(\"exampleCertificate\", new CertificateArgs
 {
 DomainName = \"example.com\",
 ValidationMethod = \"EMAIL\"
 });

 var certCertificateValidation = new CertificateValidation(\"cert\", new CertificateValidationArgs
 {
 CertificateArn = exampleCertificate.Arn,
 });
});

```
```yaml
resources:
 exampleCertificate:
 type: aws.acm.Certificate
 properties:
 domainName: \"example.com\"
 validationMethod: \"EMAIL\"
 certCertificateValidation:
 type: aws.acm.CertificateValidation
 properties:
 certificateArn: ${exampleCertificate.arn}
```
{{% /example %}}

{{% /examples %}}

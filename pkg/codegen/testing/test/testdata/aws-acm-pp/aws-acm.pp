plainDomain = "example.com"
albCertificateArn = "someArn"
zoneId = "eu"

resource "acm" "aws:acm/certificate:Certificate" {
  options {
    range = albCertificateArn == "" ? 1 : 0
  }
  domainName       = plainDomain
  validationMethod = "DNS"
}

resource "dnsAcmValidation" "aws:route53/record:Record" {
  options {
    range = albCertificateArn == "" ? [ for dvo in acm[0].domainValidationOptions : {
      name   = dvo.resourceRecordName
      record = dvo.resourceRecordValue
      type   = dvo.resourceRecordType
    }] : []
  }
  name    = range.value.name
  type    = range.value.type
  zoneId  = zoneId
  records = [range.value.record]
  ttl     = 60
}

resource "acmValidation" "aws:acm/certificateValidation:CertificateValidation" {
  options {
    range = albCertificateArn == "" ? 1 : 0
  }
  certificateArn        = acm[0].arn
  validationRecordFqdns = [for record in dnsAcmValidation : record.fqdn]
}
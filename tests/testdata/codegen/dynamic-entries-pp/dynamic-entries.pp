config "domainName" "string" {
  default     = ""
  description = "A domain name for which the certificate should be issued"
}
config "validationMethod" "string" {
  default     = "DNS"
  description = "Which method to use for validation. DNS or EMAIL are valid, NONE can be used for certificates that were imported into ACM and then into Terraform."
}
config "validationOption" "any" {
  default = {}
}

resource "certificate" "aws:acm/certificate:Certificate" {
  validationOptions = [for entry in entries(validationOption) : {
    domainName       = entry.value["domain_name"]
    validationDomain = entry.value["validation_domain"]
  }]
  domainName              = domainName
  validationMethod        = validationMethod
}

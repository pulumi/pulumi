// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import "strings"

// PulumiPublishedBeforeRegistry checks if name could be a valid package, even though it
// hasn't been published in the Pulumi package registry.
//
// Put another way, for `pulumi plugin install resource <name>` to succeed, either name
// needs to be in the Pulumi registry OR PulumiPublishedBeforeRegistry(name) needs to be
// true.
//
// PulumiPublishedBeforeRegistry exists only for backwards compatibility, and should never
// need to be updated. All future resource packages published by Pulumi should be
// published in the Pulumi registry, and so should not need to be added to this exception
// list.
func PulumiPublishedBeforeRegistry(name string) bool {
	_, ok := preRegistry[strings.ToLower(name)]
	return ok
}

var preRegistry = map[string]struct{}{
	"terraform-module":                     {},
	"eks":                                  {},
	"wavefront":                            {},
	"vsphere":                              {},
	"venafi":                               {},
	"tls":                                  {},
	"sumologic":                            {},
	"tailscale":                            {},
	"oci":                                  {},
	"spotinst":                             {},
	"gcp":                                  {},
	"xyz":                                  {},
	"snowflake":                            {},
	"rancher2":                             {},
	"sdwan":                                {},
	"slack":                                {},
	"signalfx":                             {},
	"vault":                                {},
	"scm":                                  {},
	"random":                               {},
	"splunk":                               {},
	"rabbitmq":                             {},
	"pagerduty":                            {},
	"postgresql":                           {},
	"openstack":                            {},
	"okta":                                 {},
	"opsgenie":                             {},
	"mysql":                                {},
	"mongodbatlas":                         {},
	"null":                                 {},
	"meraki":                               {},
	"ns1":                                  {},
	"minio":                                {},
	"nomad":                                {},
	"newrelic":                             {},
	"mailgun":                              {},
	"linode":                               {},
	"azure":                                {},
	"kong":                                 {},
	"ise":                                  {},
	"keycloak":                             {},
	"junipermist":                          {},
	"awsx":                                 {},
	"kafka":                                {},
	"gitlab":                               {},
	"http":                                 {},
	"harness":                              {},
	"hcloud":                               {},
	"github":                               {},
	"kubernetes-coredns":                   {},
	"fastly":                               {},
	"f5bigip":                              {},
	"external":                             {},
	"ec":                                   {},
	"docker":                               {},
	"datadog":                              {},
	"digitalocean":                         {},
	"dnsimple":                             {},
	"dbtcloud":                             {},
	"databricks":                           {},
	"consul":                               {},
	"confluentcloud":                       {},
	"azuredevops":                          {},
	"cloudngfwaws":                         {},
	"cloudamqp":                            {},
	"azuread":                              {},
	"cloudinit":                            {},
	"alicloud":                             {},
	"aws-apigateway":                       {},
	"cloudflare":                           {},
	"artifactory":                          {},
	"auth0":                                {},
	"archive":                              {},
	"aiven":                                {},
	"akamai":                               {},
	"aws":                                  {},
	"aws-native":                           {},
	"command":                              {},
	"docker-build":                         {},
	"kubernetes":                           {},
	"kubernetes-cert-manager":              {},
	"terraform":                            {},
	"kubernetes-ingress-nginx":             {},
	"azure-native-sdk":                     {},
	"azure-native":                         {},
	"pulumiservice":                        {},
	"tls-self-signed-cert":                 {},
	"terraform-provider":                   {},
	"synced-folder":                        {},
	"metabase":                             {},
	"google-cloud-static-website":          {},
	"gcp-global-cloudrun":                  {},
	"azure-static-website":                 {},
	"azure-quickstart-acr-geo-replication": {},
	"aws-static-website":                   {},
	"aws-quickstart-vpc":                   {},
	"aws-quickstart-redshift":              {},
	"aws-quickstart-aurora-postgres":       {},
	"aws-iam":                              {},
	"run-my-darn-container":                {},
	"aws-s3-replicated-bucket":             {},
	"aws-miniflux":                         {},
	"str":                                  {},
	"std":                                  {},
	"google-native":                        {},
	"auto-deploy":                          {},
	"hyperv":                               {},
	"local":                                {},
	"rke":                                  {},
	"onelogin":                             {},
	"import-google-cloud-account-scraper":  {},
	"libvirt":                              {},
	"civo":                                 {},
	"azure-justrun":                        {},
	"cloud":                                {},
	"kubernetesx":                          {},
	"pascal":                               {},
	"equinix-metal":                        {},
	"hubspot":                              {},
	"kubernetes-crds":                      {},
	"package":                              {},
	"confluent":                            {},
	"yandex":                               {},
	"azure-quickstart-compute":             {},
	"ucloud":                               {},
	"terraform-template":                   {},
	"epsagon":                              {},
	"packet":                               {},
	"openfaas":                             {},
}

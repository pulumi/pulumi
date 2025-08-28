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

// IsPreGitHubRegistryPackage checks if a package name corresponds to a Pulumi package
// that was published before the GitHub Public Pulumi Registry existed and should be
// resolved using the traditional GitHub/CDN fallback mechanism.
//
// Pulumi has three distinct registry systems:
// (A) GitHub Public Pulumi Registry - https://github.com/pulumi/templates
// (B) Pulumi IDP Registry - The cloud-hosted registry in Pulumi Cloud
// (C) Pre-GitHub Public Pulumi Registry - Known packages that existed before (A)
//
// The resolution fallback chain is: (B) → (A) for packages in this list → local project packages
//
// This function serves a critical purpose in the plugin resolution fallback chain:
// When IDP Registry lookup fails, we only attempt the expensive and error-prone
// GitHub/CDN fallback for packages that we KNOW exist there. This prevents
// users from seeing confusing "404 GitHub API" errors for truly non-existent
// packages, and instead shows clear "package not found in registry" messages.
//
// The packages in this list were published to github.com/pulumi/pulumi-<name>
// before the GitHub Public Pulumi Registry existed and should continue to work via
// the traditional download mechanism.
//
// This list should never be updated - all new Pulumi packages are published
// to the registry systems.
func IsPreGitHubRegistryPackage(name string) bool {
	_, ok := preGitHubPublicRegistryPackages[strings.ToLower(name)]
	return ok
}

var preGitHubPublicRegistryPackages = map[string]struct{}{
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

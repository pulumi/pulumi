// Copyright 2016-2017, Pulumi Corporation
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

package arn

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"

	aws "github.com/pulumi/lumi/lib/aws/rpc"
)

const (
	arnPrefix                       = "arn"
	arnDefaultPartition             = "aws"
	arnDefaultResourceSeparator     = ":"
	arnAlternativeResourceSeparator = "/"
	arnPathDelimiter                = "/"
)

// ARN is a string representation of an Amazon Resource Name (ARN).
type ARN string

// New creates a new AWS ARN string from the given account and service information.  For more information about the ARN
// format, see http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html.
func New(service string, region string, accountID string, res string) ARN {
	parts := Parts{
		Partition: arnDefaultPartition,
		Service:   service,
		Region:    region,
		AccountID: accountID,
		Resource:  res,
	}
	return parts.ARN()
}

// NewResource creates a new AWS ARN string from the given service information.  This handles the canonical use case
// of delimiting the resource type and name with a ":".  For "/" delimiters, see the NewResourceAltARN function.
func NewResource(service string, region string, accountID string, restype string, resname string) ARN {
	return New(service, region, accountID, restype+arnDefaultResourceSeparator+resname)
}

// NewResourceAlt creates a new AWS ARN string from the given service information.  This handles the canonical use
// case of delimiting the resource type, but, unlike NewResourceARN, uses "/" as the delimiter instead of ":".
func NewResourceAlt(service string, region string, accountID string, restype string, resname string) ARN {
	return New(service, region, accountID, restype+arnAlternativeResourceSeparator+resname)
}

// NewID is the same as New except that it returns a string suitable as a Lumi resource ID.
func NewID(service string, region string, accountID string, res string) resource.ID {
	return resource.ID(New(service, region, accountID, res))
}

// NewResourceID is the same as NewResource except that it returns a string suitable as a Lumi resource ID.
func NewResourceID(service string, region string, accountID string, restype string, resname string) resource.ID {
	return resource.ID(NewResource(service, region, accountID, restype, resname))
}

// NewResourceAltID is the same as NewResourceAltARN except that it returns a string suitable as a Lumi resource ID.
func NewResourceAltID(service string, region string, accountID string, restype string, resname string) resource.ID {
	return resource.ID(NewResourceAlt(service, region, accountID, restype, resname))
}

// RPC turns an ARN into its marshalable form.
func (arn ARN) RPC() aws.ARN { return aws.ARN(arn) }

// Parse turns a string formatted ARN into the consistuent ARN parts for inspection purposes.
func (arn ARN) Parse() (Parts, error) {
	var parts Parts
	ps := strings.Split(string(arn), ":")
	if len(ps) == 0 {
		return parts, errors.Errorf("Missing ARN prefix of '%v:'", arnPrefix)
	} else if ps[0] != arnPrefix {
		return parts, errors.Errorf("Unexpected ARN prefix of '%v'; expected '%v'", ps[0], arnPrefix)
	}
	if len(ps) > 1 {
		parts.Partition = ps[1]
	}
	if len(ps) > 2 {
		parts.Service = ps[2]
	}
	if len(ps) > 3 {
		parts.Region = ps[3]
	}
	if len(ps) > 4 {
		parts.AccountID = ps[4]
	}
	if len(ps) > 5 {
		parts.Resource = ps[5]
	}
	for i := 6; i < len(ps); i++ {
		parts.Resource = parts.Resource + ":" + ps[i]
	}
	return parts, nil
}

// ParseResourceName parses an entire ARN and extracts its resource name part, returning an error if the process
// fails or if the resource name is missing.
func (arn ARN) ParseResourceName() (string, error) {
	parts, err := arn.Parse()
	if err != nil {
		return "", err
	}
	resname := parts.ResourceName()
	if resname == "" {
		return "", errors.Errorf("Missing resource name in ARN '%v'", arn)
	}
	return resname, nil
}

// ParseResourceNamePair parses an entire ARN and extracts its resource name part, returning an error if the process
// fails or if the resource name is missing.
func (arn ARN) ParseResourceNamePair() (string, string, error) {
	parts, err := arn.Parse()
	if err != nil {
		return "", "", err
	}
	name1, name2 := parts.ResourceNamePair()
	if name1 == "" || name2 == "" {
		return "", "", errors.Errorf("ARN did not contain a name pair '%v'", arn)
	}
	return name1, name2, nil
}

// Parts is a structure containing an ARN's distinct parts.  Normally ARNs flow around as strings in the format
// described at http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html, however, when it comes time
// to creating or inspecting them, this first class structure can come in handy.
type Parts struct {
	Partition string
	Service   string
	Region    string
	AccountID string
	Resource  string
}

// ARN turns the ARN parts into a single string in the canonical ARN format.  Some or all parts may be missing.
func (a Parts) ARN() ARN {
	return ARN(fmt.Sprintf("%v:%v:%v:%v:%v:%v",
		arnPrefix, a.Partition, a.Service, a.Region, a.AccountID, a.Resource))
}

// ResourceType parses an ARN and returns the resource type.  This detects both kinds of resource delimiters (":" and
// "/"), although, if called on an ARN not formatted as type, delimiter, and then ID, this may return the wrong thing.
func (a Parts) ResourceType() string {
	res := a.Resource
	if idx := strings.Index(res, arnDefaultResourceSeparator); idx != -1 {
		return res[:idx]
	}
	if idx := strings.Index(res, arnAlternativeResourceSeparator); idx != -1 {
		return res[:idx]
	}
	return res
}

// ResourceName parses an ARN and returns the resource name.  This detects both kinds of resource delimiters (":" and
// "/"); if called on an ARN not formatted as type, delimiter, followed by ID, this may return the wrong thing.
func (a Parts) ResourceName() string {
	res := a.Resource
	if idx := strings.Index(res, arnDefaultResourceSeparator); idx != -1 {
		return res[idx+1:]
	}
	if idx := strings.Index(res, arnAlternativeResourceSeparator); idx != -1 {
		return res[idx+1:]
	}
	return res
}

// ResourceNamePair parses an ARN in the format of a name pair delimited by "/".  An example is an S3 object, whose
// whose ARN is of the form "arn:aws:s3:::bucket_name/key_name".  This function will return the "bucket_name" and
// "key_name" parts as independent parts, for convenient parsing as a single atomic operation.
func (a Parts) ResourceNamePair() (string, string) {
	name := a.ResourceName()
	if ix := strings.Index(name, "/"); ix != -1 {
		return name[:ix], name[ix+1:]
	}
	return name, ""
}

// ParseResourceName parses a resource ID to obtain its resource name which, for the entire AWS package, is the ARN.
func ParseResourceName(id resource.ID) (string, error) {
	return ARN(id).ParseResourceName()
}

// ParseResourceNamePair parses a resource ID to obtain a pair of resource names.  See ResourceNamePair for details.
func ParseResourceNamePair(id resource.ID) (string, string, error) {
	return ARN(id).ParseResourceNamePair()
}

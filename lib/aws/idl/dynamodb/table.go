// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamodb

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Table resource creates an AWS DynamoDB Table.  For more information, see
// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/.
type Table struct {
	idl.NamedResource
	HashKey       string      `lumi:"hashKey,replaces"`
	Attributes    []Attribute `lumi:"attributes"`
	ReadCapacity  float64     `lumi:"readCapacity"`
	WriteCapacity float64     `lumi:"writeCapacity"`
	RangeKey      *string     `lumi:"rangeKey,optional,replaces"`
	TableName     *string     `lumi:"tableName,optional,replaces"`

	// TODO:
	// LocalSecondaryIndexes
	// GlobalSecondaryIndexes
	// StreamSpecification
}

// Attribute is a DynamoDB Table Attribute definition.
type Attribute struct {
	// Name of the DynamoDB Table Attribute.
	Name string `lumi:"name"`
	// Type of the DynamoDB Table Attribute.  You can specify S for string data, N for numeric data, or B for binary data.
	Type AttributeType `lumi:"type"`
}

// AttributeType represents the types of DynamoDB Table Attributes.
type AttributeType string

const (
	StringAttribute AttributeType = "S"
	NumberAttribute AttributeType = "N"
	BinaryAttribute AttributeType = "B"
)

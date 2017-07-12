// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package dynamodb

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Table resource creates an AWS DynamoDB Table.  For more information, see
// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/.
type Table struct {
	idl.NamedResource
	HashKey                string                  `lumi:"hashKey,replaces"`
	Attributes             []Attribute             `lumi:"attributes"`
	ReadCapacity           float64                 `lumi:"readCapacity"`
	WriteCapacity          float64                 `lumi:"writeCapacity"`
	RangeKey               *string                 `lumi:"rangeKey,optional,replaces"`
	TableName              *string                 `lumi:"tableName,optional,replaces"`
	GlobalSecondaryIndexes *[]GlobalSecondaryIndex `lumi:"globalSecondaryIndexes,optional"`

	// TODO[pulumi/lumi#216]:
	// LocalSecondaryIndexes
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

// A GlobalSecondaryIndex represents an alternative index at DynamoDB Table
type GlobalSecondaryIndex struct {
	IndexName        string         `lumi:"indexName"`
	HashKey          string         `lumi:"hashKey"`
	RangeKey         *string        `lumi:"rangeKey,optional"`
	ReadCapacity     float64        `lumi:"readCapacity"`
	WriteCapacity    float64        `lumi:"writeCapacity"`
	NonKeyAttributes []string       `lumi:"nonKeyAttributes"`
	ProjectionType   ProjectionType `lumi:"projectionType"`
}

// ProjectionType represents the types of DynamoDB Table Attributes.
type ProjectionType string

const (
	KeysOnlyProjection ProjectionType = "KEYS_ONLY"
	IncludeProjection  ProjectionType = "INCLUDE"
	AllProjection      ProjectionType = "ALL"
)

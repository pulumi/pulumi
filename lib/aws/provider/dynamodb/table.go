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
	"crypto/sha1"
	"errors"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	awsdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/dynamodb"
)

const TableToken = dynamodb.TableToken

// constants for the various table limits.
const (
	minTableName              = 3
	maxTableName              = 255
	minTableAttributeName     = 1
	maxTableAttributeName     = 255
	minReadCapacity           = 1
	minWriteCapacity          = 1
	maxGlobalSecondaryIndexes = 5
)

// NewTableProvider creates a provider that handles DynamoDB Table operations.
func NewTableProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &tableProvider{ctx}
	return dynamodb.NewTableProvider(ops)
}

type tableProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *tableProvider) Check(ctx context.Context, obj *dynamodb.Table) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError

	if name := obj.TableName; name != nil {
		if len(*name) < minTableName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), dynamodb.Table_Name,
					fmt.Errorf("less than minimum length of %v", minTableName)))
		}
		if len(*name) > maxTableName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), dynamodb.Table_Name,
					fmt.Errorf("exceeded maximum length of %v", maxTableName)))
		}
		// TODO: check the vailidity of names ([a-zA-Z0-9_.-]+).
	}

	if obj.ReadCapacity < minReadCapacity {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), dynamodb.Table_ReadCapacity,
				fmt.Errorf("less than minimum of %v", minReadCapacity)))
	}
	if obj.WriteCapacity < minWriteCapacity {
		failures = append(failures,
			mapper.NewFieldErr(reflect.TypeOf(obj), dynamodb.Table_WriteCapacity,
				fmt.Errorf("less than minimum of %v", minWriteCapacity)))
	}

	for _, attribute := range obj.Attributes {
		if len(attribute.Name) < minTableAttributeName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(attribute), dynamodb.Attribute_Name,
					fmt.Errorf("less than minimum length of %v", minTableAttributeName)))
		}
		if len(attribute.Name) > maxTableAttributeName {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(attribute), dynamodb.Attribute_Name,
					fmt.Errorf("exceeded maximum length of %v", maxTableAttributeName)))
		}
		switch attribute.Type {
		case "S", "N", "B":
			break
		default:
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(attribute), dynamodb.Attribute_Type,
					fmt.Errorf("not one of valid values S (string), N (number) or B (binary)")))
		}
	}

	if obj.GlobalSecondaryIndexes != nil {
		gsis := *obj.GlobalSecondaryIndexes
		if len(gsis) > maxGlobalSecondaryIndexes {
			failures = append(failures,
				mapper.NewFieldErr(reflect.TypeOf(obj), dynamodb.Table_GlobalSecondaryIndexes,
					fmt.Errorf("more than %v global secondary indexes requested", maxGlobalSecondaryIndexes)))
		}
		for _, gsi := range gsis {
			name := gsi.IndexName
			if len(name) < minTableName {
				failures = append(failures,
					mapper.NewFieldErr(reflect.TypeOf(gsi), dynamodb.GlobalSecondaryIndex_IndexName,
						fmt.Errorf("less than minimum length of %v", minTableName)))
			}
			if len(name) > maxTableName {
				failures = append(failures,
					mapper.NewFieldErr(reflect.TypeOf(gsi), dynamodb.GlobalSecondaryIndex_IndexName,
						fmt.Errorf("exceeded maximum length of %v", maxTableName)))
			}
			if gsi.ReadCapacity < minReadCapacity {
				failures = append(failures,
					mapper.NewFieldErr(reflect.TypeOf(gsi), dynamodb.GlobalSecondaryIndex_ReadCapacity,
						fmt.Errorf("less than minimum of %v", minReadCapacity)))
			}
			if gsi.WriteCapacity < minWriteCapacity {
				failures = append(failures,
					mapper.NewFieldErr(reflect.TypeOf(gsi), dynamodb.GlobalSecondaryIndex_WriteCapacity,
						fmt.Errorf("less than minimum of %v", minWriteCapacity)))
			}
		}
	}

	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *tableProvider) Create(ctx context.Context, obj *dynamodb.Table) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var id resource.ID
	if obj.TableName != nil {
		id = resource.ID(*obj.TableName)
	} else {
		id = resource.NewUniqueHexID(obj.Name+"-", maxTableName, sha1.Size)
	}

	var attributeDefinitions []*awsdynamodb.AttributeDefinition
	for _, attr := range obj.Attributes {
		attributeDefinitions = append(attributeDefinitions, &awsdynamodb.AttributeDefinition{
			AttributeName: aws.String(attr.Name),
			AttributeType: aws.String(string(attr.Type)),
		})
	}

	fmt.Printf("Creating DynamoDB Table '%v' with name '%v'\n", obj.Name, id)
	keySchema := []*awsdynamodb.KeySchemaElement{
		{
			AttributeName: aws.String(obj.HashKey),
			KeyType:       aws.String("HASH"),
		},
	}
	if obj.RangeKey != nil {
		keySchema = append(keySchema, &awsdynamodb.KeySchemaElement{
			AttributeName: obj.RangeKey,
			KeyType:       aws.String("RANGE"),
		})
	}
	create := &awsdynamodb.CreateTableInput{
		TableName:            id.StringPtr(),
		AttributeDefinitions: attributeDefinitions,
		KeySchema:            keySchema,
		ProvisionedThroughput: &awsdynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(int64(obj.ReadCapacity)),
			WriteCapacityUnits: aws.Int64(int64(obj.WriteCapacity)),
		},
	}
	if obj.GlobalSecondaryIndexes != nil {
		var gsis []*awsdynamodb.GlobalSecondaryIndex
		for _, gsi := range *obj.GlobalSecondaryIndexes {
			keySchema := []*awsdynamodb.KeySchemaElement{
				{
					AttributeName: aws.String(gsi.HashKey),
					KeyType:       aws.String("HASH"),
				},
			}
			if gsi.RangeKey != nil {
				keySchema = append(keySchema, &awsdynamodb.KeySchemaElement{
					AttributeName: gsi.RangeKey,
					KeyType:       aws.String("RANGE"),
				})
			}
			gsis = append(gsis, &awsdynamodb.GlobalSecondaryIndex{
				IndexName: aws.String(gsi.IndexName),
				KeySchema: keySchema,
				ProvisionedThroughput: &awsdynamodb.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(int64(gsi.ReadCapacity)),
					WriteCapacityUnits: aws.Int64(int64(gsi.WriteCapacity)),
				},
				Projection: &awsdynamodb.Projection{
					NonKeyAttributes: aws.StringSlice(gsi.NonKeyAttributes),
					ProjectionType:   aws.String(string(gsi.ProjectionType)),
				},
			})
		}
		create.GlobalSecondaryIndexes = gsis
	}

	// Now go ahead and perform the action.
	if _, err := p.ctx.DynamoDB().CreateTable(create); err != nil {
		return "", err
	}

	// Wait for the table to be ready and then return the ID (just its name).
	fmt.Printf("DynamoDB Table created: %v; waiting for it to become active\n", id)
	if err := p.waitForTableState(id, true); err != nil {
		return "", err
	}
	return id, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *tableProvider) Get(ctx context.Context, id resource.ID) (*dynamodb.Table, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *tableProvider) InspectChange(ctx context.Context, id resource.ID,
	old *dynamodb.Table, new *dynamodb.Table, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *tableProvider) Update(ctx context.Context, id resource.ID,
	old *dynamodb.Table, new *dynamodb.Table, diff *resource.ObjectDiff) error {

	// Note: Changing dynamodb.Table_Attributes alone does not trigger an update on the resource, it must be changed
	// along with using the new attributes in an index.  The latter will process the update.

	// Per DynamoDB documention at http://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_UpdateTable.html:

	// You can only perform one of the following operations at once:
	// * Modify the provisioned throughput settings of the table.
	// * Enable or disable Streams on the table.
	// * Remove a global secondary index from the table.
	// * Create a new global secondary index on the table. Once the index begins backfilling, you can use
	//   UpdateTable to perform other operations.

	// So we have to serialize each of the requested updates and potentially make multiple calls to UpdateTable, waiting
	// for the Table to reach the ready state between calls.

	// First modify provisioned throughput if needed.
	if diff.Changed(dynamodb.Table_ReadCapacity) || diff.Changed(dynamodb.Table_WriteCapacity) {
		fmt.Printf("Updating provisioned capacity for DynamoDB Table %v\n", id.String())
		update := &awsdynamodb.UpdateTableInput{
			TableName: id.StringPtr(),
			ProvisionedThroughput: &awsdynamodb.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(int64(new.ReadCapacity)),
				WriteCapacityUnits: aws.Int64(int64(new.WriteCapacity)),
			},
		}
		if err := p.updateTable(id, update); err != nil {
			return err
		}
	}

	// Next, delete and create global secondary indexes.
	if diff.Changed(dynamodb.Table_GlobalSecondaryIndexes) {
		newGlobalSecondaryIndexes := newGlobalSecondaryIndexHashSet(new.GlobalSecondaryIndexes)
		oldGlobalSecondaryIndexes := newGlobalSecondaryIndexHashSet(old.GlobalSecondaryIndexes)
		d := oldGlobalSecondaryIndexes.Diff(newGlobalSecondaryIndexes)
		// First, add any new indexes
		for _, o := range d.Adds() {
			gsi := o.(globalSecondaryIndexHash).item
			fmt.Printf("Adding new global secondary index %v for DynamoDB Table %v\n", gsi.IndexName, id.String())
			keySchema := []*awsdynamodb.KeySchemaElement{
				{
					AttributeName: aws.String(gsi.HashKey),
					KeyType:       aws.String("HASH"),
				},
			}
			if gsi.RangeKey != nil {
				keySchema = append(keySchema, &awsdynamodb.KeySchemaElement{
					AttributeName: gsi.RangeKey,
					KeyType:       aws.String("RANGE"),
				})
			}
			var attributeDefinitions []*awsdynamodb.AttributeDefinition
			for _, attr := range new.Attributes {
				attributeDefinitions = append(attributeDefinitions, &awsdynamodb.AttributeDefinition{
					AttributeName: aws.String(attr.Name),
					AttributeType: aws.String(string(attr.Type)),
				})
			}
			update := &awsdynamodb.UpdateTableInput{
				TableName:            aws.String(id.String()),
				AttributeDefinitions: attributeDefinitions,
				GlobalSecondaryIndexUpdates: []*awsdynamodb.GlobalSecondaryIndexUpdate{
					{
						Create: &awsdynamodb.CreateGlobalSecondaryIndexAction{
							IndexName: aws.String(gsi.IndexName),
							KeySchema: keySchema,
							ProvisionedThroughput: &awsdynamodb.ProvisionedThroughput{
								ReadCapacityUnits:  aws.Int64(int64(gsi.ReadCapacity)),
								WriteCapacityUnits: aws.Int64(int64(gsi.WriteCapacity)),
							},
							Projection: &awsdynamodb.Projection{
								NonKeyAttributes: aws.StringSlice(gsi.NonKeyAttributes),
								ProjectionType:   aws.String(string(gsi.ProjectionType)),
							},
						},
					},
				},
			}
			if err := p.updateTable(id, update); err != nil {
				return err
			}
		}
		// Next, modify provisioned throughput on any updated indexes
		for _, o := range d.Updates() {
			gsi := o.(globalSecondaryIndexHash).item
			fmt.Printf("Updating capacity for global secondary index %v for DynamoDB Table %v\n", gsi.IndexName, id.String())
			update := &awsdynamodb.UpdateTableInput{
				TableName: aws.String(id.String()),
				GlobalSecondaryIndexUpdates: []*awsdynamodb.GlobalSecondaryIndexUpdate{
					{
						Update: &awsdynamodb.UpdateGlobalSecondaryIndexAction{
							IndexName: aws.String(gsi.IndexName),
							ProvisionedThroughput: &awsdynamodb.ProvisionedThroughput{
								ReadCapacityUnits:  aws.Int64(int64(gsi.ReadCapacity)),
								WriteCapacityUnits: aws.Int64(int64(gsi.WriteCapacity)),
							},
						},
					},
				},
			}
			if err := p.updateTable(id, update); err != nil {
				return err
			}
		}
		// Finally, delete and removed indexes
		for _, o := range d.Deletes() {
			gsi := o.(globalSecondaryIndexHash).item
			fmt.Printf("Deleting global secondary index %v for DynamoDB Table %v\n", gsi.IndexName, id.String())
			update := &awsdynamodb.UpdateTableInput{
				TableName: aws.String(id.String()),
				GlobalSecondaryIndexUpdates: []*awsdynamodb.GlobalSecondaryIndexUpdate{
					{
						Delete: &awsdynamodb.DeleteGlobalSecondaryIndexAction{
							IndexName: aws.String(gsi.IndexName),
						},
					},
				},
			}
			if err := p.updateTable(id, update); err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *tableProvider) Delete(ctx context.Context, id resource.ID) error {
	// First, perform the deletion.
	fmt.Printf("Deleting DynamoDB Table '%v'\n", id)
	succ, err := awsctx.RetryUntilLong(
		p.ctx,
		func() (bool, error) {
			_, err := p.ctx.DynamoDB().DeleteTable(&awsdynamodb.DeleteTableInput{
				TableName: id.StringPtr(),
			})
			if err != nil {
				if awsctx.IsAWSError(err, "ResourceNotFoundException") {
					return true, nil
				} else if awsctx.IsAWSError(err, "ResourceInUseException") {
					return false, nil
				}
				return false, err // anything else is a real error; propagate it.
			}
			return true, nil
		},
	)
	if err != nil {
		return err
	}
	if !succ {
		return fmt.Errorf("DynamoDB table '%v' could not be deleted", id)
	}

	// Wait for the table to actually become deleted before returning.
	fmt.Printf("DynamoDB Table delete request submitted; waiting for it to delete\n")
	return p.waitForTableState(id, false)
}

func (p *tableProvider) updateTable(id resource.ID, update *awsdynamodb.UpdateTableInput) error {
	succ, err := awsctx.RetryUntil(
		p.ctx,
		func() (bool, error) {
			_, err := p.ctx.DynamoDB().UpdateTable(update)
			if err != nil {
				if erraws, iserraws := err.(awserr.Error); iserraws {
					if erraws.Code() == "ResourceNotFoundException" || erraws.Code() == "ResourceInUseException" {
						fmt.Printf("Waiting to update resource '%v': %v", id, erraws.Message())
						return false, nil
					}
				}
				return false, err // anything else is a real error; propagate it.
			}
			return true, nil
		},
	)
	if err != nil {
		return err
	}
	if !succ {
		return fmt.Errorf("DynamoDB table '%v' could not be updated", id)
	}
	if err := p.waitForTableState(id, true); err != nil {
		return err
	}
	return nil
}

func (p *tableProvider) waitForTableState(id resource.ID, exist bool) error {
	succ, err := awsctx.RetryUntilLong(
		p.ctx,
		func() (bool, error) {
			description, err := p.ctx.DynamoDB().DescribeTable(&awsdynamodb.DescribeTableInput{
				TableName: id.StringPtr(),
			})

			if err != nil {
				if awsctx.IsAWSError(err, "ResourceNotFoundException") {
					// The table is missing; if exist==false, we're good, otherwise keep retrying.
					return !exist, nil
				}
				return false, err // anything other than "ResourceNotFoundException" is a real error; propagate it.
			}

			if exist && aws.StringValue(description.Table.TableStatus) != "ACTIVE" {
				return false, nil
			}

			// If we got here, the table was found and was ACTIVE if exist is true; if exist==true, we're good; else, keep retrying.
			return exist, nil
		},
	)
	if err != nil {
		return err
	}
	if !succ {
		var reason string
		if exist {
			reason = "active"
		} else {
			reason = "deleted"
		}
		return fmt.Errorf("DynamoDB table '%v' did not become %v", id, reason)
	}
	return nil
}

type globalSecondaryIndexHash struct {
	item dynamodb.GlobalSecondaryIndex
}

var _ awsctx.Hashable = globalSecondaryIndexHash{}

func (option globalSecondaryIndexHash) HashKey() awsctx.Hash {
	return awsctx.Hash(option.item.IndexName)
}
func (option globalSecondaryIndexHash) HashValue() awsctx.Hash {
	return awsctx.Hash(string(int(option.item.ReadCapacity)) + ":" + string(int(option.item.WriteCapacity)))
}
func newGlobalSecondaryIndexHashSet(options *[]dynamodb.GlobalSecondaryIndex) *awsctx.HashSet {
	set := awsctx.NewHashSet()
	if options == nil {
		return set
	}
	for _, option := range *options {
		set.Add(globalSecondaryIndexHash{option})
	}
	return set
}

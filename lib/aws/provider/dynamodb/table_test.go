// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package dynamodb

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awsdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	"github.com/pulumi/lumi/lib/aws/rpc/dynamodb"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	t.Parallel()

	prefix := resource.NewUniqueHex("lumitest", 20, 20)
	ctx := testutil.CreateContext(t)
	defer func() {
		err := cleanup(prefix, ctx)
		assert.Nil(t, err)
	}()

	testutil.ProviderTestSimple(t, NewTableProvider(ctx), TableToken, []interface{}{
		&dynamodb.Table{
			Name: aws.String(prefix),
			Attributes: []dynamodb.Attribute{
				{Name: "Album", Type: "S"},
				{Name: "Artist", Type: "S"},
				{Name: "Sales", Type: "N"},
			},
			HashKey:       "Album",
			RangeKey:      aws.String("Artist"),
			ReadCapacity:  2,
			WriteCapacity: 2,
			GlobalSecondaryIndexes: &[]dynamodb.GlobalSecondaryIndex{
				{
					IndexName:        "myGSI",
					HashKey:          "Sales",
					RangeKey:         aws.String("Artist"),
					ReadCapacity:     1,
					WriteCapacity:    1,
					NonKeyAttributes: []string{"Album"},
					ProjectionType:   "INCLUDE",
				},
			},
		},
		&dynamodb.Table{
			Name: aws.String(prefix),
			Attributes: []dynamodb.Attribute{
				{Name: "Album", Type: "S"},
				{Name: "Artist", Type: "S"},
				{Name: "NumberOfSongs", Type: "N"},
				{Name: "Sales", Type: "N"},
			},
			HashKey:       "Album",
			RangeKey:      aws.String("Artist"),
			ReadCapacity:  1,
			WriteCapacity: 1,
			GlobalSecondaryIndexes: &[]dynamodb.GlobalSecondaryIndex{
				{
					IndexName:        "myGSI",
					HashKey:          "Sales",
					RangeKey:         aws.String("Artist"),
					ReadCapacity:     1,
					WriteCapacity:    1,
					NonKeyAttributes: []string{"Album"},
					ProjectionType:   "INCLUDE",
				},
				{
					IndexName:        "myGSI2",
					HashKey:          "NumberOfSongs",
					RangeKey:         aws.String("Sales"),
					NonKeyAttributes: []string{"Album", "Artist"},
					ProjectionType:   "INCLUDE",
					ReadCapacity:     1,
					WriteCapacity:    1,
				},
			},
		},
	})
}

func cleanup(prefix string, ctx *awsctx.Context) error {
	fmt.Printf("Cleaning up tables with prefix: %v\n", prefix)
	list, err := ctx.DynamoDB().ListTables(&awsdynamodb.ListTablesInput{})
	if err != nil {
		return err
	}
	cleaned := 0
	for _, table := range list.TableNames {
		if strings.HasPrefix(aws.StringValue(table), prefix) {
			if _, delerr := ctx.DynamoDB().DeleteTable(&awsdynamodb.DeleteTableInput{
				TableName: table,
			}); delerr != nil {
				return delerr
			}
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v tables\n", cleaned)
	return nil
}

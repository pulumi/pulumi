package dynamodb

import (
	"fmt"
	"testing"

	"encoding/json"

	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/dynamodb"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"github.com/stretchr/testify/assert"
)

const TABLENAMEPREFIX = "lumitest"

func marshal(table dynamodb.Table) (*structpb.Struct, error) {
	byts, err := json.Marshal(table)
	if err != nil {
		return nil, err
	}
	var obj map[string]interface{}
	err = json.Unmarshal(byts, &obj)
	if err != nil {
		return nil, err
	}
	props := resource.NewPropertyMapFromMap(obj)
	return resource.MarshalProperties(nil, props, resource.MarshalOptions{}), nil
}

func cleanup(ctx *awsctx.Context) {
	fmt.Printf("Cleaning up tables with prefix: %v\n", TABLENAMEPREFIX)
	list, err := ctx.DynamoDB().ListTables(&awsdynamodb.ListTablesInput{})
	if err != nil {
		return
	}
	cleaned := 0
	for _, table := range list.TableNames {
		if strings.HasPrefix(aws.StringValue(table), TABLENAMEPREFIX) {
			ctx.DynamoDB().DeleteTable(&awsdynamodb.DeleteTableInput{
				TableName: table,
			})
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v tables\n", cleaned)
}

func Test(t *testing.T) {
	// Create a TableProvider
	ctx, err := awsctx.New()
	assert.Nil(t, err, "expected no error getting AWS context")
	tableProvider := NewTableProvider(ctx)

	defer cleanup(ctx)

	// Table to create
	tablename := TABLENAMEPREFIX
	table := dynamodb.Table{
		Name: &tablename,
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
	}
	props, err := marshal(table)
	if !assert.NoError(t, err, "expected no error marshaling object to protobuf") {
		return
	}
	checkResp, err := tableProvider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(TableToken),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error checking table") {
		return
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")

	// Invoke Create request
	resp, err := tableProvider.Create(nil, &lumirpc.CreateRequest{
		Type:       string(TableToken),
		Properties: props,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return
	}
	if !assert.NotNil(t, resp, "expected a non-nil response") {
		return
	}

	id := resp.Id
	assert.Contains(t, id, "lumitest", "expected resource ID to contain `lumitest`")

	// Table for update
	tablename2 := "lumitest"
	table2 := dynamodb.Table{
		Name: &tablename2,
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
	}
	props2, err := marshal(table2)
	if !assert.NoError(t, err, "expected no error marshaling object to protobuf") {
		return
	}
	checkResp, err = tableProvider.Check(nil, &lumirpc.CheckRequest{
		Type:       string(TableToken),
		Properties: props2,
	})
	if !assert.NoError(t, err, "expected no error checking table") {
		return
	}
	assert.Equal(t, 0, len(checkResp.Failures), "expected no check failures")

	// Invoke Update request
	_, err = tableProvider.Update(nil, &lumirpc.UpdateRequest{
		Type: string(TableToken),
		Id:   id,
		Olds: props,
		News: props2,
	})
	if !assert.NoError(t, err, "expected no error creating resource") {
		return
	}

	// Invoke the Delete request
	_, err = tableProvider.Delete(nil, &lumirpc.DeleteRequest{
		Type: string(TableToken),
		Id:   id,
	})
	if !assert.NoError(t, err, "expected no error deleting resource") {
		return
	}
}

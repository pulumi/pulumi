package dynamodb

import (
	"fmt"
	"testing"

	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsdynamodb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	"github.com/pulumi/lumi/lib/aws/rpc/dynamodb"
	"github.com/stretchr/testify/assert"
)

const RESOURCEPREFIX = "lumitest"

func Test(t *testing.T) {
	t.Parallel()

	ctx, err := awsctx.New()
	assert.Nil(t, err, "expected no error getting AWS context")

	cleanup(ctx)

	testutil.ProviderTestSimple(t, NewTableProvider(ctx), TableToken, []interface{}{
		&dynamodb.Table{
			Name: aws.String(RESOURCEPREFIX),
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
			Name: aws.String(RESOURCEPREFIX),
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

func cleanup(ctx *awsctx.Context) {
	fmt.Printf("Cleaning up tables with prefix: %v\n", RESOURCEPREFIX)
	list, err := ctx.DynamoDB().ListTables(&awsdynamodb.ListTablesInput{})
	if err != nil {
		return
	}
	cleaned := 0
	for _, table := range list.TableNames {
		if strings.HasPrefix(aws.StringValue(table), RESOURCEPREFIX) {
			ctx.DynamoDB().DeleteTable(&awsdynamodb.DeleteTableInput{
				TableName: table,
			})
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v tables\n", cleaned)
}

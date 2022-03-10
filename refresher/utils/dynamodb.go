package utils

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	goKitDynamo "github.com/infralight/go-kit/db/dynamo"
	"github.com/infralight/go-kit/helpers"
	goKitTypes "github.com/infralight/go-kit/types"
	"strings"
	"time"
)

func GetAtrsFromDynamo(accountId, tableName string, atrs []string, client *goKitDynamo.Client) ([]string, error) {
	items := dynamodb.KeysAndAttributes{}

	for _, atr := range atrs {
		items.Keys = append(items.Keys, map[string]*dynamodb.AttributeValue{
			"AccountId": {
				S: aws.String(accountId),
			},
			"ATR": {
				S: aws.String(atr),
			},
		})
	}

	batchItems, err := client.BatchGetItems(&items, tableName)
	atrItems := make([]goKitDynamo.EngineAccumulatorItem, 0, len(atrs))
	bytesItems, err := json.Marshal(batchItems)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytesItems, &atrItems)
	if err != nil {
		return nil, err
	}
	currentTimestamp := time.Now().Unix()
	formattedAtrs := make([]string, 0, len(atrItems))
	for _, item := range atrItems {
		if int64(item.ExpirationDate) > currentTimestamp {
			formattedAtrs = append(formattedAtrs, fmt.Sprintf("%s-%s", item.AccountId, item.ATR))
		}

	}
	return formattedAtrs, nil
}

func WriteAtrsToDynamo(accountId, tableName string, atrs []string, dynamoTTL int, client *goKitDynamo.Client) error {
	if len(atrs) == 0 {
		return nil
	}
	var items []*dynamodb.WriteRequest
	for _, atr := range atrs {
		var providerType string
		splited := strings.Split(atr, "-")
		assetType := splited[1]
		if strings.HasPrefix(assetType, "aws") {
			providerType = "aws"
		} else if strings.HasPrefix(assetType, "kubernetes") {
			providerType = "k8s"
		}

		currentTimeStamp := time.Now().Unix()
		expirationDate := goKitTypes.ToString(currentTimeStamp + int64(dynamoTTL) + 5)
		LastTriggered := goKitTypes.ToString(currentTimeStamp)
		itemToCreate := map[string]*dynamodb.AttributeValue{
			"AccountId": {
				S: aws.String(accountId),
			},
			"ATR": {
				S: aws.String(atr),
			},
			"ProviderType": {
				S: aws.String(providerType),
			},
			"ExpirationDate": {
				N: aws.String(expirationDate),
			},
			"LastTriggered": {
				N: aws.String(LastTriggered),
			},
		}

		items = append(items, &dynamodb.WriteRequest{PutRequest: &dynamodb.PutRequest{Item: itemToCreate}})
	}

	err := client.BatchPutItems(items, tableName)
	if err != nil {
		return err
	}
	return nil

}

func DiffDynamoItems(dynamoItems []string, atrsToTrigger []string, accountId string) ([]string, error) {

	filteredAtrs := make([]string, 0, len(atrsToTrigger))
	for _, atr := range atrsToTrigger {
		formattedAtr := fmt.Sprintf("%s-%s", accountId, atr)
		if !helpers.StringSliceContains(dynamoItems, formattedAtr) {
			filteredAtrs = append(filteredAtrs, atr)
		}
	}
	return filteredAtrs, nil

}

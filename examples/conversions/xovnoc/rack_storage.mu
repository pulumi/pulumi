package xovnoc

import "aws/dynamodb"
import "aws/s3"

service rackStorage {
    new() {
        // S3 buckets:
        registryBucket := new s3.Bucket {
            deletionPolicy: "Retain"
            accessControl: "Private"
        }

        settings := new s3.Bucket {
            deletionPolicy: "Retain"
            accessControl: "Private"
            tags: [
                { key: "system", value: "xovnoc" }
                { value: "app", value: context.stack.name }
            ]
        }

        // DynamoDB tables:
        dynamoBuilds := new dynamodb.Table {
            tableName: context.stack.name + "-builds"
            attributeDefinitions: [
                { attributeName: "id", attributeType: "S" }
                { attributeName: "app", attributeType: "S" }
                { attributeName: "created", attributeType: "S" }
            ]
            keySchema: [{ attributeName: "id", keyType: "HASH" }]
            globalSecondaryIndexes: [{
                indexName: "app.created"
                keySchema: [
                    { attributeName: "app", keyType: "HASH" }
                    { attributeName: "created", keyType: "RANGE" }
                ]
                projection: { projectionType: "ALL" }
                provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
            }]
            provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
        }

        dynamoReleases := new dynamodb.Table {
            tableName: context.stack.name + "-releases"
            attributeDefinitions: [
                { attributeName: "id", attributeType: "S" }
                { attributeName: "app", attributeType: "S" }
                { attributeName: "created", attributeType: "S" }
            ]
            keySchema: [{ attributeName: "id", keyType: "HASH" }]
            globalSecondaryIndexes: [{
                indexName: "app.created"
                keySchema: [
                    { attributeName: "app", keyType: "HASH" }
                    { attributeName: "created", keyType: "RANGE" }
                ]
                projection: { projectionType: "ALL" }
                provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
            }]
            provisionedThroughput: { readCapacityUnits: 5, writeCapacityUnits: 5 }
        }
    }
}


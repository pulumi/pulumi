// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as aws from "@lumi/aws";
import * as lumi from "@lumi/lumi";

let music = new aws.dynamodb.Table("music", {
  attributes: [
    { name: "Album", type: "S" },
    { name: "Artist", type: "S" },
    { name: "NumberOfSongs", type: "N" },
    { name: "Sales", type: "N" },
  ],
  hashKey: "Album",
  rangeKey: "Artist",
  readCapacity: 1,
  writeCapacity: 1,
  globalSecondaryIndexes: [
    {
      indexName: "myGSI",
      hashKey: "Sales",
      rangeKey: "Artist",
      readCapacity: 1,
      writeCapacity: 1,
      nonKeyAttributes: ["NumberOfSongs", "Album"],
      projectionType: "INCLUDE",
    },
    {
      indexName: "myGSI2",
      hashKey: "NumberOfSongs",
      rangeKey: "Sales",
      nonKeyAttributes: ["Album", "Artist"],
      projectionType: "INCLUDE",
      readCapacity: 2,
      writeCapacity: 2,
    },
  ],
});

let hello = "Hello, world!";
let lambda = new aws.serverless.Function(
  "mylambda",
  { policies: [aws.iam.AWSLambdaFullAccess] },
  (event, context, callback) => {
    console.log("Music table hash key is: " + music.hashKey);
    console.log("Invoked function: " + context.invokedFunctionArn);
    callback(null, {
      statusCode: 200,
      body: hello + "\n\nSucceeed with " + context.getRemainingTimeInMillis() + "ms remaining.\n",
    });
  },
);

let api = new aws.serverless.API("frontend");
api.route("GET", "/bambam", lambda);
api.route("PUT", "/bambam", lambda);
let stage = api.publish();


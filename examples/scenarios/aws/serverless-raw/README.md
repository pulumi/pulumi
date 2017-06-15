# examples/scenarios/aws/serverless-raw

An example using some serverless AWS resources, currently including:

* AWS Lambda Function
* AWS IAM Role
* AWS DynamoDB Table
* AWS APIGateway RestAPI

To deploy (and re-deploy):

```bash
lumijs && lumi deploy
```

To invoke the lambda deployed by the script:

```bash
./invoke.sh
```


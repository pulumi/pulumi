# examples/scenarios/aws/serverless

An example using some serverless AWS resources, currently including:

* AWS DynamoDB Table: Based on http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html#cfn-dynamodb-table-examples
* FunctionX: A high level wrapper for the following resources.
  * AWS Lambda Function
  * AWS IAM Role
* AWS IAM Managed Policies  

To deploy (and re-deploy):
```bash
lumijs && lumi deploy
```

To invoke the lambda deployed by the script:
```bash
export LAMBDA=mylambda-d12c918004e # replace with your lambda's name from the `lumi deploy` output
aws lambda invoke --function-name $LAMBDA --log-type Tail out.txt | jq '.LogResult' -r | base64 --decode
```

#!/bin/sh
echo "Enter your Lambda's name (usually in the form mylambda-f0780fd95f4) followed by [ENTER]: "
read LAMBDA
echo "Attempting to invoke $LAMBDA ..."
aws lambda invoke --function-name $LAMBDA --log-type Tail out.txt | jq '.LogResult' -r | base64 --decode


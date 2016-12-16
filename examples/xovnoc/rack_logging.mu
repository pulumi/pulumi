module xovnoc

import "aws/iam"
import "aws/logs"
import "aws/lambda"

service rackLogging {
    new() {
        logGroup := new logs.LogGroup {}
        logSubscriptionFilterFunction := new lambda.Function {
            code: // TODO
            handler: "index.handler"
            memorySize: 128
            role: logSubscriptionFilterRole
            runtime: "nodejs"
            timeout: 30
        }
        logSubscriptionFilter := new logs.SubscriptionFilter {
            destination: logSubscriptionFilterFunction
            filterPattern: ""
            logGroup: logGroup
        }
        logSubscriptionFilterPermission := new lambda.Permission {
            action: "lambda:InvokeFunction"
            functionName: logSubscriptionFilterFunction
            principal: "logs." + context.region + ".amazonaws.com"
            sourceAccount: context.accountId
            source: logGroup
        }
    }

    properties {
        logSubscriptionFilterRole: iam.Role
    }
}


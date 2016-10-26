// This code is the moral equivalent of Mulesoft's example "code" here:
// https://github.com/mulesoft/twilio-connector/blob/master/example-send-sms-interact-with-mongo/src/main/app/mule-config.xml

var mu = require("mu");
var mongo = require("mu-mongo");
var twilio = require("mu-twilio");

var clients = mongo.connect("clients");
clients.forEach(client =>
    twilio.sendSmsMessage({
        to: cust.phone,
        body: `Hi ${client.name}! Your account balance is USD${client.accountBalance}`,
    });
);


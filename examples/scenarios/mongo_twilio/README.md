# mongo_twilio

This sample sends an SMS via Twilio to any new customer entry added to a MongoDB database.

This is derived from Mulesoft's sample app here:
https://github.com/mulesoft/twilio-connector/tree/master/example-send-sms-interact-with-mongo

Assuming we are deploying to AWS, running `$ mu deploy app.js` creates a new CloudFormation stack.  That stack includes
a single lambda that is triggered off modifications to the `clients` MongoDB database.  This database is not provisioned
by this stack; instead, we use name-based discovery to find an existing database configured in the environment.

In fact, many configuration entries are automatically plumbed around in this example, which are passed manually in the
Mulesoft example.  For instance, the Twilio account SID and "from" phone number, are configured per stack deployment.


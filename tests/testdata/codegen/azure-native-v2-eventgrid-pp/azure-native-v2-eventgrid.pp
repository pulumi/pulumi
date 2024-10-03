resource "example" "azure-native:eventgrid:EventSubscription" {
    destination = {
        endpointType = "EventHub"
        resourceId = "example"
    }
    expirationTimeUtc = "example"
    scope = "example"
}
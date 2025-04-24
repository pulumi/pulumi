import pulumi
import pulumi_azure_native as azure_native

example = azure_native.eventgrid.EventSubscription("example",
    destination={
        "endpoint_type": "EventHub",
        "resource_id": "example",
    },
    expiration_time_utc="example",
    scope="example")

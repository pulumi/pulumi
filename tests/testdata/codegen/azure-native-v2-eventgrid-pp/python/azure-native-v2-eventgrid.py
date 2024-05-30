import pulumi
import pulumi_azure_native as azure_native

example = azure_native.eventgrid.EventSubscription("example",
    expiration_time_utc="example",
    scope="example")

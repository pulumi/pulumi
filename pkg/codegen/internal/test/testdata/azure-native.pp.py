import pulumi
import pulumi_azure_native as azure_native

front_door = azure_native.network.FrontDoor("frontDoor", routing_rules=[azure_native.network.RoutingRuleArgs(
    route_configuration={
        "odata_type": "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
        "backend_pool": {
            "id": "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
        },
    },
)])
endpoint = azure_native.cdn.Endpoint("endpoint",
    delivery_policy=azure_native.cdn.EndpointPropertiesUpdateParametersDeliveryPolicyArgs(
        rules=[azure_native.cdn.DeliveryRuleArgs(
            actions=[
                azure_native.cdn.DeliveryRuleCacheExpirationActionArgs(
                    name="CacheExpiration",
                    parameters=azure_native.cdn.CacheExpirationActionParametersArgs(
                        cache_behavior="Override",
                        cache_duration="10:10:09",
                        cache_type="All",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                    ),
                ),
                azure_native.cdn.DeliveryRuleResponseHeaderActionArgs(
                    name="ModifyResponseHeader",
                    parameters=azure_native.cdn.HeaderActionParametersArgs(
                        header_action="Overwrite",
                        header_name="Access-Control-Allow-Origin",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value="*",
                    ),
                ),
                azure_native.cdn.DeliveryRuleRequestHeaderActionArgs(
                    name="ModifyRequestHeader",
                    parameters=azure_native.cdn.HeaderActionParametersArgs(
                        header_action="Overwrite",
                        header_name="Accept-Encoding",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value="gzip",
                    ),
                ),
            ],
            conditions=[azure_native.cdn.DeliveryRuleRemoteAddressConditionArgs(
                name="RemoteAddress",
                parameters=azure_native.cdn.RemoteAddressMatchConditionParametersArgs(
                    match_values=[
                        "192.168.1.0/24",
                        "10.0.0.0/24",
                    ],
                    negate_condition=True,
                    odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
                    operator="IPMatch",
                ),
            )],
            name="rule1",
            order=1,
        )],
    ),
    endpoint_name="endpoint1",
    is_compression_enabled=True,
    is_http_allowed=True,
    is_https_allowed=True,
    location="WestUs")

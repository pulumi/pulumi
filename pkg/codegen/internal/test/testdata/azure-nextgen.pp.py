import pulumi
import pulumi_azure_nextgen as azure_nextgen

front_door = azure_nextgen.FrontDoor("frontDoor", routing_rules=[azure_nextgen.network.RoutingRuleArgs(
    route_configuration={
        "odata_type": "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
        "backend_pool": {
            "id": "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
        },
    },
)])
endpoint = azure_nextgen.Endpoint("endpoint",
    delivery_policy=azure_nextgen.cdn.EndpointPropertiesUpdateParametersDeliveryPolicyArgs(
        rules=[azure_nextgen.cdn.DeliveryRuleArgs(
            actions=[
                azure_nextgen.cdn.DeliveryRuleCacheExpirationActionArgs(
                    name="CacheExpiration",
                    parameters=azure_nextgen.cdn.CacheExpirationActionParametersArgs(
                        cache_behavior="Override",
                        cache_duration="10:10:09",
                        cache_type="All",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                    ),
                ),
                azure_nextgen.cdn.DeliveryRuleResponseHeaderActionArgs(
                    name="ModifyResponseHeader",
                    parameters=azure_nextgen.cdn.HeaderActionParametersArgs(
                        header_action="Overwrite",
                        header_name="Access-Control-Allow-Origin",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value="*",
                    ),
                ),
                azure_nextgen.cdn.DeliveryRuleRequestHeaderActionArgs(
                    name="ModifyRequestHeader",
                    parameters=azure_nextgen.cdn.HeaderActionParametersArgs(
                        header_action="Overwrite",
                        header_name="Accept-Encoding",
                        odata_type="#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        value="gzip",
                    ),
                ),
            ],
            conditions=[azure_nextgen.cdn.DeliveryRuleRemoteAddressConditionArgs(
                name="RemoteAddress",
                parameters=azure_nextgen.cdn.RemoteAddressMatchConditionParametersArgs(
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

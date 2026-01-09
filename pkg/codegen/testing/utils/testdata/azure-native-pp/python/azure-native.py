import pulumi
import pulumi_azure_native as azure_native

front_door = azure_native.network.FrontDoor("frontDoor",
    resource_group_name="someGroupName",
    routing_rules=[{
        "route_configuration": {
            "odata_type": "#Microsoft.Azure.FrontDoor.Models.FrontdoorForwardingConfiguration",
            "backend_pool": {
                "id": "/subscriptions/subid/resourceGroups/rg1/providers/Microsoft.Network/frontDoors/frontDoor1/backendPools/backendPool1",
            },
        },
    }])
endpoint = azure_native.cdn.Endpoint("endpoint",
    origins=[],
    delivery_policy={
        "rules": [{
            "actions": [
                {
                    "name": "CacheExpiration",
                    "parameters": {
                        "cache_behavior": azure_native.cdn.CacheBehavior.OVERRIDE,
                        "cache_duration": "10:10:09",
                        "cache_type": azure_native.cdn.CacheType.ALL,
                        "odata_type": "#Microsoft.Azure.Cdn.Models.DeliveryRuleCacheExpirationActionParameters",
                    },
                },
                {
                    "name": "ModifyResponseHeader",
                    "parameters": {
                        "header_action": azure_native.cdn.HeaderAction.OVERWRITE,
                        "header_name": "Access-Control-Allow-Origin",
                        "odata_type": "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        "value": "*",
                    },
                },
                {
                    "name": "ModifyRequestHeader",
                    "parameters": {
                        "header_action": azure_native.cdn.HeaderAction.OVERWRITE,
                        "header_name": "Accept-Encoding",
                        "odata_type": "#Microsoft.Azure.Cdn.Models.DeliveryRuleHeaderActionParameters",
                        "value": "gzip",
                    },
                },
            ],
            "conditions": [{
                "name": "RemoteAddress",
                "parameters": {
                    "match_values": [
                        "192.168.1.0/24",
                        "10.0.0.0/24",
                    ],
                    "negate_condition": True,
                    "odata_type": "#Microsoft.Azure.Cdn.Models.DeliveryRuleRemoteAddressConditionParameters",
                    "operator": azure_native.cdn.RemoteAddressOperator.IP_MATCH,
                },
            }],
            "name": "rule1",
            "order": 1,
        }],
    },
    endpoint_name="endpoint1",
    is_compression_enabled=True,
    is_http_allowed=True,
    is_https_allowed=True,
    location="WestUs",
    profile_name="profileName",
    resource_group_name="resourceGroupName")

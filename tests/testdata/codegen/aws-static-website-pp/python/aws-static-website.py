import pulumi
import pulumi_aws_static_website as aws_static_website

website_resource = aws_static_website.Website("websiteResource",
    site_path="string",
    index_html="string",
    cache_ttl=0,
    cdn_args={
        "cloudfront_function_associations": [{
            "event_type": "string",
            "function_arn": "string",
        }],
        "forwarded_values": {
            "cookies": {
                "forward": "string",
                "whitelisted_names": ["string"],
            },
            "query_string": False,
            "headers": ["string"],
            "query_string_cache_keys": ["string"],
        },
        "lambda_function_associations": [{
            "event_type": "string",
            "lambda_arn": "string",
            "include_body": False,
        }],
    },
    certificate_arn="string",
    error404="string",
    add_website_version_header=False,
    price_class="string",
    atomic_deployments=False,
    subdomain="string",
    target_domain="string",
    with_cdn=False,
    with_logs=False)

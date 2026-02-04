import pulumi
import pulumi_dns as dns

subscription = dns.Subscription("subscription", domains=[
    "example.com",
    "test.com",
])
record = []
def create_record(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        record.append(dns.Record(f"record-{range['key']}", name=range["value"].record_name))

subscription.challenges.apply(create_record)

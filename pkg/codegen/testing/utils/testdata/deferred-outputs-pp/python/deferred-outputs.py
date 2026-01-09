import pulumi
from first import First
from second import Second

second_password_length, resolve_second_password_length = pulumi.deferred_output()
first = First("first", {
    'passwordLength': second_password_length})
second = Second("second", {
    'petName': first.pet_name})
resolve_second_password_length(second.password_length)
looping_over_many, resolve_looping_over_many = pulumi.deferred_output()
another = First("another", {
    'passwordLength': looping_over_many.apply(lambda looping_over_many: len(looping_over_many))})
many = []
for range in [{"value": i} for i in range(0, 10)]:
    many.append(Second(f"many-{range['value']}", {
        'petName': another.pet_name    }))
resolve_looping_over_many(pulumi.Output.from_input([v["passwordLength"] for _, v in many]))

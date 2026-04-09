import pulumi
import third_party as other

other_1 = other.Thing("Other", idea="Support Third Party")
question = other.module.Object("Question", answer=42)
question2 = other.module.sub.Object("Question2", answer=24)
provider = other.Provider("Provider", object_prop={
    "prop1": "foo",
    "prop2": "bar",
    "prop3": "fizz",
})

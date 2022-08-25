import pulumi
import third_party as other

other = other.Thing("Other", idea="Support Third Party")
question = other.module.Object("Question", answer=42)
provider = other.Provider("Provider", name="foo")

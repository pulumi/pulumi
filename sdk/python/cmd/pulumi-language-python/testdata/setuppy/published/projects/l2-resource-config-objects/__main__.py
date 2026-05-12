import pulumi
import pulumi_primitive as primitive

config = pulumi.Config()
plain_number_array = config.require_object("plainNumberArray")
plain_boolean_map = config.require_object("plainBooleanMap")
secret_number_array = config.require_secret_object("secretNumberArray")
secret_boolean_map = config.require_secret_object("secretBooleanMap")
plain = primitive.Resource("plain",
    boolean=True,
    float=3.5,
    integer=3,
    string="plain",
    number_array=plain_number_array,
    boolean_map=plain_boolean_map)
secret = primitive.Resource("secret",
    boolean=True,
    float=3.5,
    integer=3,
    string="secret",
    number_array=secret_number_array,
    boolean_map=secret_boolean_map)

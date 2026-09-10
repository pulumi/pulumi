import pulumi
from primitiveComponent import PrimitiveComponent

config = pulumi.Config()
plain_bool = config.require_bool("plainBool")
plain_number = config.require_float("plainNumber")
plain_integer = config.require_int("plainInteger")
plain_string = config.require("plainString")
secret_bool = config.require_secret_bool("secretBool")
secret_number = config.require_secret_float("secretNumber")
secret_integer = config.require_secret_int("secretInteger")
secret_string = config.require_secret("secretString")
plain = PrimitiveComponent("plain", {
    'boolean': plain_bool, 
    'float': plain_number, 
    'integer': plain_integer, 
    'string': plain_string})
secret = PrimitiveComponent("secret", {
    'boolean': secret_bool, 
    'float': secret_number, 
    'integer': secret_integer, 
    'string': secret_string})

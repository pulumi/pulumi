import pulumi
from primitiveComponent import PrimitiveComponent

config = pulumi.Config()
plain_number_array = config.require_object("plainNumberArray")
plain_boolean_map = config.require_object("plainBooleanMap")
secret_number_array = config.require_secret_object("secretNumberArray")
secret_boolean_map = config.require_secret_object("secretBooleanMap")
plain = PrimitiveComponent("plain", {
    'numberArray': plain_number_array, 
    'booleanMap': plain_boolean_map})
secret = PrimitiveComponent("secret", {
    'numberArray': secret_number_array, 
    'booleanMap': secret_boolean_map})

import pulumi
import pulumi_primitive as primitive

config = pulumi.Config()
plain_bool = config.require_bool("plainBool")
plain_number = config.require_float("plainNumber")
plain_integer = config.require_int("plainInteger")
plain_string = config.require("plainString")
plain_numeric_string = config.require("plainNumericString")
secret_number = config.require_secret_float("secretNumber")
secret_integer = config.require_secret_int("secretInteger")
secret_string = config.require_secret("secretString")
secret_numeric_string = config.require_secret("secretNumericString")
plain_values = primitive.Resource("plainValues",
    boolean=plain_string == "true",
    float=float(plain_integer),
    integer=int(plain_numeric_string),
    string=str(plain_number),
    number_array=[
        float(plain_integer),
        float(plain_numeric_string),
        plain_number,
    ],
    boolean_map={
        "fromBool": plain_bool,
        "fromString": plain_string == "true",
    })
secret_values = primitive.Resource("secretValues",
    boolean=secret_string.apply(lambda x: x == "true"),
    float=secret_integer.apply(lambda x: float(x)),
    integer=secret_numeric_string.apply(lambda x: int(x)),
    string=secret_number.apply(lambda x: str(x)),
    number_array=[
        float(plain_integer),
        float(plain_numeric_string),
        plain_number,
    ],
    boolean_map={
        "fromBool": plain_bool,
        "fromString": plain_string == "true",
    })
invoke_result = primitive.invoke_output(boolean=plain_string == "true",
    float=float(plain_integer),
    integer=int(plain_numeric_string),
    string="true" if plain_bool else "false",
    number_array=[
        float(plain_integer),
        float(plain_numeric_string),
        plain_number,
    ],
    boolean_map={
        "fromBool": plain_bool,
        "fromString": plain_string == "true",
    })
invoke_values = primitive.Resource("invokeValues",
    boolean=invoke_result.boolean,
    float=invoke_result.float,
    integer=invoke_result.integer,
    string=invoke_result.string,
    number_array=invoke_result.number_array,
    boolean_map=invoke_result.boolean_map)

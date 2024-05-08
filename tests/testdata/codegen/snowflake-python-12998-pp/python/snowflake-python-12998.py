import pulumi
import pulumi_snowflake as snowflake

table_association = snowflake.TagAssociation("tableAssociation", object_identifiers=[snowflake.TagAssociationObjectIdentifierArgs(
    name=test["name"],
    database=snowflake_database["value"]["name"],
    schema=snowflake_schema["value"]["name"],
)])

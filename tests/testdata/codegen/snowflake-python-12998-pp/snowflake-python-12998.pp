resource tableAssociation "snowflake:index/tagAssociation:TagAssociation" {
  objectIdentifiers =[{
    name     = test.name,
    database = snowflake_database.value.name,
    schema   = snowflake_schema.value.name
  }]
}
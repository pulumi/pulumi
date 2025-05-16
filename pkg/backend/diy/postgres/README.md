# PostgreSQL Backend for Pulumi

This package provides a PostgreSQL-based backend implementation for Pulumi state storage. It stores Pulumi state in a PostgreSQL database table, allowing you to use PostgreSQL as your state storage mechanism.

## Features

- Store Pulumi state in a PostgreSQL database
- Use standard PostgreSQL authentication and connection options
- Supports all Pulumi stack operations
- Configurable table name

## Connection String Format

The PostgreSQL backend connection string follows the standard PostgreSQL connection string format with a `postgres://` prefix:

```
postgres://username:password@hostname:port/database?param1=value1&param2=value2
```

## Configuration Options

The following query parameters are supported in the connection string:

- `table`: The name of the table to use for state storage (default: `pulumi_state`)
- All standard PostgreSQL connection parameters (sslmode, connect_timeout, etc.)

## Usage

To use PostgreSQL as your Pulumi state backend, use the `--backend` flag with the `pulumi` command:

```bash
pulumi login --backend postgres://username:password@hostname:port/database?sslmode=disable

# Or with a specific table name
pulumi login --backend postgres://username:password@hostname:port/database?sslmode=disable&table=mystatestable
```

## Environment Setup

For most secure setups, use environment variables to store your PostgreSQL credentials:

```bash
export PGUSER=username
export PGPASSWORD=password
export PGHOST=hostname
export PGPORT=5432
export PGDATABASE=database

# Then login with minimal connection string
pulumi login --backend postgres://
```

## Table Schema

The PostgreSQL backend will automatically create the necessary table for state storage. The table has the following schema:

```sql
CREATE TABLE IF NOT EXISTS pulumi_state (
    key TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS pulumi_state_key_prefix_idx ON pulumi_state (key text_pattern_ops);
```

## Security Considerations

- Always use SSL connections in production (`sslmode=require` or `sslmode=verify-full`)
- Create a dedicated database user with limited permissions for Pulumi state storage
- Ensure your database is properly secured with appropriate network access controls
- Enable database backups to prevent data loss

## Limitations

- Signed URLs (used for state permalinks in the Pulumi CLI) are not supported with the PostgreSQL backend
- Performance may be slower compared to cloud-specific object storage backends for very large states 
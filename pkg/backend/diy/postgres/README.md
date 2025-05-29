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

## PostgreSQL Connection Parameters
The following PostgreSQL connection parameters can be included in the connection string query parameters:

### SSL and Security Parameters
- `sslmode`: SSL connection mode (`disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`)
- `sslcert`: Client certificate file path
- `sslkey`: Client private key file path
- `sslrootcert`: Root certificate file path
- `sslcrl`: Certificate revocation list file path
- `sslcompression`: Enable SSL compression (`0` or `1`)
- `sslsni`: Enable SSL Server Name Indication (`0` or `1`)

### Connection and Timeout Parameters
- `connect_timeout`: Maximum time to wait for connection (seconds)
- `keepalives_idle`: Time before sending keepalive probe (seconds)
- `keepalives_interval`: Interval between keepalive probes (seconds)
- `keepalives_count`: Number of keepalive probes before giving up
- `tcp_user_timeout`: Time for transmitted data to be acknowledged (milliseconds)

### Application and Session Parameters
- `application_name`: Application name for connection identification
- `client_encoding`: Client character set encoding
- `options`: Command-line options to send to server on connection startup
- `timezone`: Session timezone setting

### Authentication Parameters
- `passfile`: Password file path (alternative to inline password)

### Connection Pool and Behavior Parameters
- `target_session_attrs`: Required session attributes (`any`, `read-write`, `read-only`, `primary`, `standby`, `prefer-standby`)
- `load_balance_hosts`: Enable connection load balancing (`disable`, `random`)
- `hostaddr`: Numeric IP address (can be used instead of or in addition to host)

### Examples with Parameters
```bash
# Basic SSL connection
postgres://user:pass@localhost:5432/mydb?sslmode=require

# Connection with custom table and SSL verification
postgres://user:pass@db.example.com:5432/pulumi?table=my_state&sslmode=verify-full&sslrootcert=/path/to/ca.crt

# Connection with timeout and keepalive settings
postgres://user:pass@localhost:5432/mydb?connect_timeout=30&keepalives_idle=600&keepalives_interval=30

# Read-only replica connection with application name
postgres://user:pass@replica.example.com:5432/mydb?target_session_attrs=read-only&application_name=pulumi-state

# Connection with custom search path and timezone
postgres://user:pass@localhost:5432/mydb?search_path=pulumi,public&timezone=UTC
```

## Usage
To use PostgreSQL as your Pulumi state backend:
```bash
pulumi login postgres://username:password@hostname:port/database
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
pulumi login postgres://
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

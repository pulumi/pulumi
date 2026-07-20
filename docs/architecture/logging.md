# Logging

Pulumi automatically logs every operation to `$PULUMI_HOME/logs`. This doc describes the technical details for that.

For automatic logs, the CLI is always solely responsible for writing the logs to disk.  Logs from plugins flow to the CLI via OTEL. The CLI spins up its own OTEL receiver for logs and passes the address for it via the `PULUMI_LOG_OTLP_ENDPOINT` env variable.

## Property values

Property values can contain secrets, and are thus treated specially.  Plugins send property values as protobuf encoded structs, with a magic prefix (`pulumiPv` as little-endian uint64).  Property values from the CLI are stored in the same way.  When the user wants to view the logs, either using `pulumi logs decrypt`, or share them using `pulumi logs share`, the property values are unmarshalled, and secrets can be redacted depending on the user's preference.

## Log sharing

For sharing logs, we provide a mechanism to automatically encrypt logs (with secrets redacted by default), using `pulumi logs share`. This command works by getting a new encryption session from the server.  It does so by calling the `/api/log-encryption-session/init` API endpoint.  This returns a session ID and an encryption key.

The log is then re-encrypted (with secrets redacted by default), and the session ID is embedded in the log.  The service is expected to provide a secured endpoint, that takes the session ID as parameter, and distributes the key to whoever has access to decrypt the shared logs.

## Old logging system

We still support the old `--logtostderr` and `-v<n>` and `--logflow` flags.  The new logging system logs the equivalent to `-v10 --logflow`.

The logs for the old logging system now also flow via otel, and secrets are redacted as long as property values are logged as such.

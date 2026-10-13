## tiger db extensions

List the PostgreSQL extensions installed in a database

### Synopsis

List the PostgreSQL extensions installed in a database service, with each
extension's version, the schema it lives in, and its description.

The service can be given by ID or name as an argument, or will use the default
service from your configuration. Read replicas aren't accepted, since a
replica's extensions are its primary's. The connection is opened in Tiger
Cloud's immutable read-only mode.

```
tiger db extensions [name-or-id] [flags]
```

### Examples

```
  # List the extensions installed in the default service
  tiger db extensions

  # List the extensions installed in a specific service
  tiger db extensions svc-12345

  # Output as JSON
  tiger db extensions svc-12345 -o json
```

### Options

```
  -h, --help            help for extensions
  -o, --output string   Output format (table, json, yaml)
      --role string     Database role/username (default "tsdbadmin")
```

### Options inherited from parent commands

```
      --analytics                 enable/disable usage analytics (default true)
      --color                     enable colored output (default true)
      --config-dir string         config directory (default "~/.config/tiger")
      --password-storage string   password storage method (keyring, pgpass, none) (default "keyring")
      --service-id string         service ID
      --version-check             check for updates on startup (default true)
```

### SEE ALSO

* [tiger db](tiger_db.md)	 - Database operations and management

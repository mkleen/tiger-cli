package common

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/timescale/tiger-cli/internal/api"
	"github.com/timescale/tiger-cli/internal/config"
)

// Extension is a PostgreSQL extension installed in a service's database.
type Extension struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Schema      string `json:"schema"`
	Description string `json:"description"`
}

// FetchExtensionsArgs configures the connection FetchExtensions opens. It
// always connects directly: the extension list is the same over the pooler,
// so pooling has nothing to offer a single catalog read.
type FetchExtensionsArgs struct {
	Role string
}

const extensionsQuery = `SELECT e.extname, e.extversion, n.nspname,
       coalesce(obj_description(e.oid, 'pg_extension'), '')
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
ORDER BY e.extname`

// FetchExtensions opens a read-only connection to a primary service and lists
// the extensions installed in its database, ordered by name. Read replicas
// aren't accepted: a replica's extensions are its primary's. It is the shared entry point for the
// `tiger db extensions` CLI command and the db_extensions MCP tool.
//
// It is a var so tests can stub the connection out and reach the callers'
// success paths, which are otherwise only reachable against a live database.
var FetchExtensions = func(ctx context.Context, cfg *config.Config, service api.Service, args FetchExtensionsArgs) ([]Extension, error) {
	target := &ConnectionTarget{ConnectionService: service, CredentialService: service}
	conn, err := connectReadOnly(ctx, cfg, target, args.Role, false)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(ctx, extensionsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch extensions: %w", err)
	}
	extensions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Extension, error) {
		var ext Extension
		err := row.Scan(&ext.Name, &ext.Version, &ext.Schema, &ext.Description)
		return ext, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch extensions: %w", err)
	}
	return extensions, nil
}

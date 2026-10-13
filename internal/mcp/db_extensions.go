package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/timescale/tiger-cli/internal/common"
	"github.com/timescale/tiger-cli/internal/util"
)

// DBExtensionsInput represents input for db_extensions
type DBExtensionsInput struct {
	ServiceID string `json:"service_id"`
	Role      string `json:"role,omitempty"`
}

func (DBExtensionsInput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[DBExtensionsInput](nil))

	schema.Properties["service_id"].Description = "Unique identifier of the service (10-character alphanumeric string). Use service_list to find service IDs. Read replica set IDs aren't accepted, since a replica's extensions are its primary's."
	schema.Properties["service_id"].Examples = []any{"e6ue9697jf", "u8me885b93"}
	schema.Properties["service_id"].Pattern = "^[a-z0-9]{10}$"

	schema.Properties["role"].Description = "Database role/username to connect as"
	schema.Properties["role"].Default = util.Must(json.Marshal("tsdbadmin"))
	schema.Properties["role"].Examples = []any{"tsdbadmin", "readonly", "postgres"}

	return schema
}

// DBExtensionsOutput represents output for db_extensions
type DBExtensionsOutput struct {
	Extensions []common.Extension `json:"extensions"`
}

func (DBExtensionsOutput) Schema() *jsonschema.Schema {
	schema := util.Must(jsonschema.For[DBExtensionsOutput](nil))

	schema.Properties["extensions"].Description = "The extensions installed in the database, ordered by name."
	setExtensionSchemaProperties(schema.Properties["extensions"].Items)

	return schema
}

func newDBExtensionsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:  toolDBExtensions,
		Title: "List Database Extensions",
		Description: `List the PostgreSQL extensions installed in a service database.

Connects to a PostgreSQL/TimescaleDB service in Tiger Cloud and returns each installed extension's name, version, schema, and description. The connection is opened in immutable read-only mode.`,
		InputSchema:  DBExtensionsInput{}.Schema(),
		OutputSchema: DBExtensionsOutput{}.Schema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:  true,
			OpenWorldHint: new(false),
			Title:         "List Database Extensions",
		},
	}
}

// handleDBExtensions handles the db_extensions MCP tool
func (s *Server) handleDBExtensions(ctx context.Context, req *mcp.CallToolRequest, input DBExtensionsInput) (*mcp.CallToolResult, DBExtensionsOutput, error) {
	cfg, client, projectID, err := s.app.GetAll()
	if err != nil {
		return nil, DBExtensionsOutput{}, err
	}

	service, err := common.GetService(ctx, client, projectID, input.ServiceID)
	if err != nil {
		return nil, DBExtensionsOutput{}, err
	}

	// A replica's extensions are its primary's, so only the primary is listed.
	if common.IsReadReplica(*service) {
		return nil, DBExtensionsOutput{}, fmt.Errorf("'%s' is a read replica; list the extensions on its primary service '%s' instead",
			input.ServiceID, util.DerefStr(service.ForkedFrom.ServiceID))
	}

	extensions, err := common.FetchExtensions(ctx, cfg, *service, common.FetchExtensionsArgs{Role: input.Role})
	if err != nil {
		return nil, DBExtensionsOutput{}, handleDatabaseError(err)
	}

	return nil, DBExtensionsOutput{Extensions: extensions}, nil
}

// setExtensionSchemaProperties enhances the schema of a common.Extension
// nested inside a tool's output schema.
func setExtensionSchemaProperties(schema *jsonschema.Schema) {
	schema.Properties["name"].Description = "Extension name, as used in CREATE EXTENSION."
	schema.Properties["name"].Examples = []any{"timescaledb", "vector", "postgis"}

	schema.Properties["version"].Description = "Installed version of the extension."
	schema.Properties["version"].Examples = []any{"2.21.0", "0.8.0"}

	schema.Properties["schema"].Description = "Schema the extension's objects are installed in."
	schema.Properties["schema"].Examples = []any{"public", "pg_catalog"}

	schema.Properties["description"].Description = "The extension's description (its COMMENT ON EXTENSION text), or an empty string when it has none."
	schema.Properties["description"].Examples = []any{
		"Enables scalable inserts and complex queries for time-series data",
		"vector data type and ivfflat and hnsw access methods",
	}
}

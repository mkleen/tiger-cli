package cmd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/timescale/tiger-cli/internal/api"
	"github.com/timescale/tiger-cli/internal/api/mocks"
	"github.com/timescale/tiger-cli/internal/common"
	"github.com/timescale/tiger-cli/internal/config"
)

// withFetchExtensions stubs common.FetchExtensions — otherwise the point where
// the command would open a real database connection — asserting the arguments
// the command passed down and returning extensions and err in their place.
func withFetchExtensions(want common.FetchExtensionsArgs, extensions []common.Extension, err error) runOption {
	return withSetup(func(t *testing.T) {
		original := common.FetchExtensions
		common.FetchExtensions = func(_ context.Context, _ *config.Config, _ api.Service, got common.FetchExtensionsArgs) ([]common.Extension, error) {
			t.Helper()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("FetchExtensions args mismatch (-want +got):\n%s", diff)
			}
			return extensions, err
		}
		t.Cleanup(func() { common.FetchExtensions = original })
	})
}

func TestDbExtensionsCmd(t *testing.T) {
	setupGetWithStatus := func(status api.DeployStatus) func(m *mocks.MockClientWithResponsesInterface) {
		return func(m *mocks.MockClientWithResponsesInterface) {
			expectResolveRef(m, "svc-12345", sampleService(func(s *api.Service) {
				s.Status = status
			}))
		}
	}

	// The fetch is stubbed for the success cases below, so they reach the
	// command's own output and flag handling without a live database.
	extensions := []common.Extension{
		{Name: "plpgsql", Version: "1.0", Schema: "pg_catalog", Description: "PL/pgSQL procedural language"},
		{Name: "timescaledb", Version: "2.21.0", Schema: "public", Description: "Enables scalable inserts and complex queries for time-series data"},
		{Name: "timescaledb_toolkit", Version: "1.21.0", Schema: "public", Description: "Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities"},
		{Name: "vector", Version: "0.8.0", Schema: "public"},
	}
	defaultArgs := common.FetchExtensionsArgs{Role: "tsdbadmin"}

	runCmdTests(t, []cmdTest{
		{
			name:    "not logged in",
			args:    []string{"db", "extensions", "svc-12345"},
			opts:    []runOption{withNotLoggedIn()},
			wantErr: notLoggedInMsg,
		},
		{
			name:    "missing service id",
			args:    []string{"db", "extensions"},
			wantErr: "service name or ID is required. Provide it as an argument or set a default with 'tiger config set service_id <service-id>'",
		},
		{
			name:    "invalid output format",
			args:    []string{"db", "extensions", "svc-12345", "-o", "csv"},
			wantErr: `invalid argument "csv" for "-o, --output" flag: invalid output format: csv (must be one of: json, yaml, table)`,
		},
		{
			// Paused readiness stops the command before any connection attempt,
			// proving the config default reached the service lookup.
			name:    "default service id from config",
			args:    []string{"db", "extensions"},
			opts:    []runOption{withConfig(map[string]any{"service_id": "svc-12345"})},
			mock:    setupGetWithStatus(api.DeployStatusPAUSED),
			wantErr: pausedMsg("svc-12345"),
		},
		{
			name: "network error",
			args: []string{"db", "extensions", "svc-12345"},
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				expectResolveRefError(m, "svc-12345", errors.New("connection refused"))
			},
			wantErr: `failed to resolve service 'svc-12345': connection refused`,
		},
		{
			name: "service not found",
			args: []string{"db", "extensions", "svc-12345"},
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				expectResolveRefNotFound(m, "svc-12345")
			},
			wantErr: "service 'svc-12345' not found",
			checks:  []checkFunc{checkExitCode(common.ExitServiceNotFound)},
		},
		{
			name: "nil response body",
			args: []string{"db", "extensions", "svc-12345"},
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				expectResolveRefStatus(m, "svc-12345", http.StatusOK, nil)
			},
			wantErr: "empty response from API",
		},
		{
			name: "read replica",
			args: []string{"db", "extensions", "rep-67890"},
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				expectResolveRef(m, "rep-67890", sampleReplica())
			},
			wantErr: "'rep-67890' is a read replica; list the extensions on its primary service 'svc-12345' instead",
		},
		{
			name:    "service paused",
			args:    []string{"db", "extensions", "svc-12345"},
			mock:    setupGetWithStatus(api.DeployStatusPAUSED),
			wantErr: pausedMsg("svc-12345"),
		},
		{
			name:    "service not ready",
			args:    []string{"db", "extensions", "svc-12345"},
			mock:    setupGetWithStatus(api.DeployStatusQUEUED),
			wantErr: notReadyMsg("svc-12345"),
		},
		{
			// A fetch failure goes through handleDatabaseError like any other.
			name:    "fetch fails",
			args:    []string{"db", "extensions", "svc-12345"},
			mock:    setupGetWithStatus(api.DeployStatusREADY),
			opts:    []runOption{withFetchExtensions(defaultArgs, nil, errors.New("failed to connect to database: no route to host"))},
			wantErr: "failed to connect to database: no route to host",
		},
		{
			name: "prints a table",
			args: []string{"db", "extensions", "svc-12345"},
			mock: setupGetWithStatus(api.DeployStatusREADY),
			opts: []runOption{withFetchExtensions(defaultArgs, extensions, nil)},
			wantStdout: `┌─────────────────────┬─────────┬────────────┬───────────────────────────────────────────────────────────────────────────────────────┐
│        NAME         │ VERSION │   SCHEMA   │                                      DESCRIPTION                                      │
├─────────────────────┼─────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────┤
│ plpgsql             │ 1.0     │ pg_catalog │ PL/pgSQL procedural language                                                          │
│ timescaledb         │ 2.21.0  │ public     │ Enables scalable inserts and complex queries for time-series data                     │
│ timescaledb_toolkit │ 1.21.0  │ public     │ Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities │
│ vector              │ 0.8.0   │ public     │                                                                                       │
└─────────────────────┴─────────┴────────────┴───────────────────────────────────────────────────────────────────────────────────────┘
`,
		},
		{
			name: "prints json",
			args: []string{"db", "extensions", "svc-12345", "-o", "json"},
			mock: setupGetWithStatus(api.DeployStatusREADY),
			opts: []runOption{withFetchExtensions(defaultArgs, extensions, nil)},
			wantStdout: `[
  {
    "name": "plpgsql",
    "version": "1.0",
    "schema": "pg_catalog",
    "description": "PL/pgSQL procedural language"
  },
  {
    "name": "timescaledb",
    "version": "2.21.0",
    "schema": "public",
    "description": "Enables scalable inserts and complex queries for time-series data"
  },
  {
    "name": "timescaledb_toolkit",
    "version": "1.21.0",
    "schema": "public",
    "description": "Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities"
  },
  {
    "name": "vector",
    "version": "0.8.0",
    "schema": "public",
    "description": ""
  }
]
`,
		},
		{
			name: "prints yaml",
			args: []string{"db", "extensions", "svc-12345", "-o", "yaml"},
			mock: setupGetWithStatus(api.DeployStatusREADY),
			opts: []runOption{withFetchExtensions(defaultArgs, extensions, nil)},
			wantStdout: `- description: PL/pgSQL procedural language
  name: plpgsql
  schema: pg_catalog
  version: "1.0"
- description: Enables scalable inserts and complex queries for time-series data
  name: timescaledb
  schema: public
  version: 2.21.0
- description: Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities
  name: timescaledb_toolkit
  schema: public
  version: 1.21.0
- description: ""
  name: vector
  schema: public
  version: 0.8.0
`,
		},
		{
			name:       "passes the role down to the fetch",
			args:       []string{"db", "extensions", "svc-12345", "--role", "reader", "-o", "json"},
			mock:       setupGetWithStatus(api.DeployStatusREADY),
			opts:       []runOption{withFetchExtensions(common.FetchExtensionsArgs{Role: "reader"}, []common.Extension{}, nil)},
			wantStdout: "[]\n",
		},
	})
}

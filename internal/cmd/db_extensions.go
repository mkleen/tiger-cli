package cmd

import (
	"fmt"
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/timescale/tiger-cli/internal/common"
	"github.com/timescale/tiger-cli/internal/util"
)

func buildDbExtensionsCmd(app *common.App) *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:     "extensions [name-or-id]",
		Aliases: []string{"ext"},
		Short:   "List the PostgreSQL extensions installed in a database",
		Long: `List the PostgreSQL extensions installed in a database service, with each
extension's version, the schema it lives in, and its description.

The service can be given by ID or name as an argument, or will use the default
service from your configuration. Read replicas aren't accepted, since a
replica's extensions are its primary's. The connection is opened in Tiger
Cloud's immutable read-only mode.`,
		Example: `  # List the extensions installed in the default service
  tiger db extensions

  # List the extensions installed in a specific service
  tiger db extensions svc-12345

  # Output as JSON
  tiger db extensions svc-12345 -o json`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: serviceRefCompletion(app),
		SilenceUsage:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, projectID, err := app.GetAll()
			if err != nil {
				return err
			}

			serviceRef, err := getServiceRef(cmd, cfg, args)
			if err != nil {
				return err
			}

			service, err := resolveService(cmd.Context(), client, projectID, serviceRef)
			if err != nil {
				return err
			}

			// A replica's extensions are its primary's, so only the primary is listed.
			if common.IsReadReplica(*service) {
				return fmt.Errorf("'%s' is a read replica; list the extensions on its primary service '%s' instead",
					service.ServiceID, util.DerefStr(service.ForkedFrom.ServiceID))
			}

			extensions, err := common.FetchExtensions(cmd.Context(), cfg, *service, common.FetchExtensionsArgs{Role: role})
			if err != nil {
				return handleDatabaseError(err, &common.ConnectionTarget{ConnectionService: *service, CredentialService: *service})
			}

			switch cfg.Output {
			case "json":
				return util.SerializeToJSON(cmd.OutOrStdout(), extensions)
			case "yaml":
				return util.SerializeToYAML(cmd.OutOrStdout(), extensions)
			default:
				return outputExtensionsTable(cmd.OutOrStdout(), extensions)
			}
		},
	}

	cmd.Flags().StringVar(&role, "role", "tsdbadmin", "Database role/username")
	cmd.Flags().VarP(new(outputFlag), "output", "o", "Output format (table, json, yaml)")
	registerFlagCompletion(cmd, "output", outputCompletion())

	return cmd
}

// outputExtensionsTable renders the installed extensions as a table.
func outputExtensionsTable(w io.Writer, extensions []common.Extension) error {
	table := tablewriter.NewWriter(w)
	table.Header("NAME", "VERSION", "SCHEMA", "DESCRIPTION")

	for _, ext := range extensions {
		table.Append(ext.Name, ext.Version, ext.Schema, ext.Description)
	}

	return table.Render()
}

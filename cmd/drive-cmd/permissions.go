package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/drive"
	u "github.com/tanq16/gdrive/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var permFlags struct {
	id       string
	permType string
	role     string
	email    string
}

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Manage file and folder permissions",
}

var permListCmd = &cobra.Command{
	Use:   "list <path>",
	Short: "List permissions for a file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], permFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		perms, err := drive.Service.Files.Get(f.Id).
			Fields("permissions(id, type, role, emailAddress)").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			u.PrintFatal("failed to get permissions", drive.HandleError(err))
		}

		if len(perms.Permissions) == 0 {
			u.PrintInfo("no permissions found")
			return
		}

		headers := []string{"ID", "TYPE", "ROLE", "EMAIL"}
		var rows [][]string
		for _, p := range perms.Permissions {
			email := p.EmailAddress
			if email == "" {
				email = "-"
			}
			rows = append(rows, []string{p.Id, p.Type, p.Role, email})
		}

		u.PrintTable(headers, rows)
	},
}

var permCreateCmd = &cobra.Command{
	Use:   "create <path>",
	Short: "Create a permission on a file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if permFlags.permType == "" || permFlags.role == "" {
			u.PrintFatal("--type and --role are required", nil)
		}

		f, err := drive.ResolveOrID(args[0], permFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		perm := &driveapi.Permission{
			Type: permFlags.permType,
			Role: permFlags.role,
		}
		if permFlags.email != "" {
			perm.EmailAddress = permFlags.email
		}

		created, err := drive.Service.Permissions.Create(f.Id, perm).
			SupportsAllDrives(true).
			Do()
		if err != nil {
			u.PrintFatal("failed to create permission", drive.HandleError(err))
		}

		u.PrintSuccess("created permission " + created.Id + " (" + created.Role + ")")
	},
}

var permDeleteCmd = &cobra.Command{
	Use:   "delete <path> <permission-id>",
	Short: "Delete a permission from a file or folder",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], permFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		permID := args[1]
		err = drive.Service.Permissions.Delete(f.Id, permID).
			SupportsAllDrives(true).
			Do()
		if err != nil {
			u.PrintFatal("failed to delete permission", drive.HandleError(err))
		}

		u.PrintSuccess("deleted permission " + permID)
	},
}

func init() {
	DriveCmd.AddCommand(permissionsCmd)
	permissionsCmd.AddCommand(permListCmd)
	permissionsCmd.AddCommand(permCreateCmd)
	permissionsCmd.AddCommand(permDeleteCmd)

	permListCmd.Flags().StringVar(&permFlags.id, "id", "", "Use file ID instead of path")
	permCreateCmd.Flags().StringVar(&permFlags.permType, "type", "", "Permission type (user, group, domain, anyone)")
	permCreateCmd.Flags().StringVar(&permFlags.role, "role", "", "Permission role (reader, writer, commenter)")
	permCreateCmd.Flags().StringVar(&permFlags.email, "email", "", "Email address for the permission")
	permDeleteCmd.Flags().StringVar(&permFlags.id, "id", "", "Use file ID instead of path")
}

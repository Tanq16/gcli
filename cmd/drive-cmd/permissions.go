package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var permListFlags struct {
	id string
}

var permCreateFlags struct {
	permType string
	role     string
	email    string
}

var permDeleteFlags struct {
	id string
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
		f, err := drive.ResolveOrID(args[0], permListFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		perms, err := drive.GetPermissions(f.Id)
		if err != nil {
			u.PrintFatal("failed to get permissions", err)
		}

		if len(perms) == 0 {
			u.PrintInfo("no permissions found")
			return
		}

		headers := []string{"ID", "TYPE", "ROLE", "EMAIL"}
		var rows [][]string
		for _, p := range perms {
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
		if permCreateFlags.permType == "" || permCreateFlags.role == "" {
			u.PrintFatal("--type and --role are required", nil)
		}

		f, err := drive.ResolveOrID(args[0], "")
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		created, err := drive.CreatePermission(f.Id, permCreateFlags.permType, permCreateFlags.role, permCreateFlags.email)
		if err != nil {
			u.PrintFatal("failed to create permission", err)
		}

		u.PrintSuccess("created permission " + created.Id + " (" + created.Role + ")")
	},
}

var permDeleteCmd = &cobra.Command{
	Use:   "delete <path> <permission-id>",
	Short: "Delete a permission from a file or folder",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], permDeleteFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		permID := args[1]
		if err := drive.DeletePermission(f.Id, permID); err != nil {
			u.PrintFatal("failed to delete permission", err)
		}

		u.PrintSuccess("deleted permission " + permID)
	},
}

func init() {
	DriveCmd.AddCommand(permissionsCmd)
	permissionsCmd.AddCommand(permListCmd)
	permissionsCmd.AddCommand(permCreateCmd)
	permissionsCmd.AddCommand(permDeleteCmd)

	permListCmd.Flags().StringVarP(&permListFlags.id, "id", "i", "", "Use file ID instead of path")
	permCreateCmd.Flags().StringVarP(&permCreateFlags.permType, "type", "t", "", "Permission type (user, group, domain, anyone)")
	permCreateCmd.Flags().StringVarP(&permCreateFlags.role, "role", "r", "", "Permission role (reader, writer, commenter)")
	permCreateCmd.Flags().StringVarP(&permCreateFlags.email, "email", "e", "", "Email address for the permission")
	permDeleteCmd.Flags().StringVarP(&permDeleteFlags.id, "id", "i", "", "Use file ID instead of path")
}

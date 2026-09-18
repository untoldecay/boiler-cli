package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func userCmds() *cobra.Command {
	c := &cobra.Command{Use: "users", Short: "Manage admin users"}

	list := &cobra.Command{Use: "list", Short: "List users", RunE: func(cmd *cobra.Command, a []string) error {
		body, err := apiRequest("GET", "/admin/users", nil)
		if err != nil {
			return err
		}
		if u, ok := body["users"]; ok {
			printJSON(u)
		} else {
			printJSON(body)
		}
		return nil
	}}
	c.AddCommand(list)

	create := &cobra.Command{Use: "create", Short: "Create a user (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		role, _ := cmd.Flags().GetString("role")
		display, _ := cmd.Flags().GetString("display-name")
		if email == "" || password == "" {
			return fmt.Errorf("--email and --password are required")
		}
		if err := mustConfirm("create user " + email + " (role: " + role + ")"); err != nil {
			return err
		}
		body, err := apiRequest("POST", "/admin/users", map[string]any{
			"email": email, "password": password, "role": role, "displayName": display,
		})
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	create.Flags().String("email", "", "user email")
	create.Flags().String("password", "", "initial password")
	create.Flags().String("role", "user", "user | admin")
	create.Flags().String("display-name", "", "display name")
	c.AddCommand(create)

	del := &cobra.Command{Use: "delete", Short: "Delete a user (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if err := mustConfirm("DELETE user " + id); err != nil {
			return err
		}
		if _, err := apiRequest("DELETE", "/admin/users/"+id, nil); err != nil {
			return err
		}
		fmt.Println("✓ Deleted user", id)
		return nil
	}}
	del.Flags().String("id", "", "user id")
	c.AddCommand(del)

	return c
}

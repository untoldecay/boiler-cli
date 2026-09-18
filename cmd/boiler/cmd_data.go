package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func dataCommands() []*cobra.Command {
	// ---- db ----
	db := &cobra.Command{Use: "db", Short: "Manage databases"}
	db.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiRequest("GET", "/admin/databases", nil)
			if err != nil {
				return err
			}
			printJSON(body["databases"])
			return nil
		},
	})
	dbCreate := &cobra.Command{
		Use:   "create",
		Short: "Create a database (critical)",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := reqStr(cmd, "name")
			if err != nil {
				return err
			}
			if err := mustConfirm("create database " + name); err != nil {
				return err
			}
			body, err := apiRequest("POST", "/admin/create-database", map[string]any{"database": name, "name": name})
			if err != nil {
				return err
			}
			fmt.Printf("✓ Created database %s\n", name)
			_ = body
			return nil
		},
	}
	dbCreate.Flags().String("name", "", "database name")
	db.AddCommand(dbCreate)
	dbDelete := &cobra.Command{
		Use:   "delete",
		Short: "Delete a database (critical)",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := reqStr(cmd, "name")
			if err != nil {
				return err
			}
			if err := mustConfirm("DELETE database " + name + " (irreversible)"); err != nil {
				return err
			}
			if _, err := apiRequest("DELETE", "/admin/databases/"+name, map[string]any{"confirm": true}); err != nil {
				return err
			}
			fmt.Printf("✓ Deleted database %s\n", name)
			return nil
		},
	}
	dbDelete.Flags().String("name", "", "database name")
	db.AddCommand(dbDelete)

	// ---- tables ----
	tables := &cobra.Command{Use: "tables", Short: "Inspect tables"}
	tablesList := &cobra.Command{
		Use:   "list",
		Short: "List tables in a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := reqStr(cmd, "db")
			if err != nil {
				return err
			}
			body, err := apiRequest("GET", "/admin/databases/"+database+"/structure", nil)
			if err != nil {
				return err
			}
			printJSON(extractTables(body))
			return nil
		},
	}
	tablesList.Flags().String("db", "", "database name")
	tables.AddCommand(tablesList)
	tablesDesc := &cobra.Command{
		Use:   "describe",
		Short: "Describe a table's columns",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := reqStr(cmd, "db")
			if err != nil {
				return err
			}
			table, err := reqStr(cmd, "table")
			if err != nil {
				return err
			}
			body, err := apiRequest("GET", "/admin/databases/"+database+"/structure", nil)
			if err != nil {
				return err
			}
			for _, t := range extractTables(body) {
				if m, ok := t.(map[string]any); ok && m["name"] == table {
					printJSON(m)
					return nil
				}
			}
			return fmt.Errorf("table %q not found in %s", table, database)
		},
	}
	tablesDesc.Flags().String("db", "", "database name")
	tablesDesc.Flags().String("table", "", "table name")
	tables.AddCommand(tablesDesc)

	// ---- rows ----
	rows := &cobra.Command{Use: "rows", Short: "Read/query rows"}
	rowsFetch := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch rows from a table",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := reqStr(cmd, "db")
			if err != nil {
				return err
			}
			table, err := reqStr(cmd, "table")
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			q := fmt.Sprintf(`SELECT * FROM "%s" LIMIT %d OFFSET %d`, table, limit, offset)
			return runSQL(database, q)
		},
	}
	rowsFetch.Flags().String("db", "", "database name")
	rowsFetch.Flags().String("table", "", "table name")
	rowsFetch.Flags().Int("limit", 50, "max rows")
	rowsFetch.Flags().Int("offset", 0, "offset")
	rows.AddCommand(rowsFetch)

	// ---- sql (generic; RBAC enforced server-side) ----
	sql := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run SQL (SELECT runs freely; writes require --confirm)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := reqStr(cmd, "db")
			if err != nil {
				return err
			}
			query := args[0]
			head := strings.ToUpper(strings.TrimSpace(query))
			readOnly := strings.HasPrefix(head, "SELECT") || strings.HasPrefix(head, "WITH") || strings.HasPrefix(head, "EXPLAIN") || strings.HasPrefix(head, "SHOW") || strings.HasPrefix(head, "VALUES")
			if !readOnly {
				if err := mustConfirm("mutating SQL on " + database + ": " + truncate(query, 80)); err != nil {
					return err
				}
			}
			return runSQL(database, query)
		},
	}
	sql.Flags().String("db", "", "database name")

	return []*cobra.Command{db, tables, rows, sql}
}

func extractTables(body map[string]any) []any {
	structure, _ := body["structure"].(map[string]any)
	if structure == nil {
		return nil
	}
	// prefer public schema; else first schema
	if pub, ok := structure["public"].(map[string]any); ok {
		if t, ok := pub["tables"].([]any); ok {
			return t
		}
	}
	for _, v := range structure {
		if s, ok := v.(map[string]any); ok {
			if t, ok := s["tables"].([]any); ok {
				return t
			}
		}
	}
	return nil
}

func runSQL(database, query string) error {
	body, err := apiRequest("POST", "/admin/execute", map[string]any{"query": query, "database": database})
	if err != nil {
		return err
	}
	if rows, ok := body["rows"]; ok {
		printJSON(map[string]any{"rows": rows, "rowCount": body["rowCount"], "command": body["command"]})
	} else {
		printJSON(body)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

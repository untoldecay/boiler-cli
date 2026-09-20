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
			tbls := extractTables(body)
			comments := tableComments(database)
			for _, t := range tbls {
				if m, ok := t.(map[string]any); ok {
					if name, ok := m["name"].(string); ok {
						if c, ok := comments[name]; ok {
							m["comment"] = c
						}
					}
				}
			}
			printJSON(tbls)
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
					if tc := tableComments(database)[table]; tc != "" {
						m["comment"] = tc
					}
					ccs := columnComments(database, table)
					if cols, ok := m["columns"].([]any); ok {
						for _, col := range cols {
							if cm, ok := col.(map[string]any); ok {
								if name, ok := cm["name"].(string); ok {
									if c, ok := ccs[name]; ok {
										cm["comment"] = c
									}
								}
							}
						}
					}
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

	rowsInsert := &cobra.Command{Use: "insert", Short: "Insert a row (--set col=val …)", RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := reqStr(cmd, "db")
		table, _ := cmd.Flags().GetString("table")
		set, _ := cmd.Flags().GetStringArray("set")
		if database == "" || table == "" || len(set) == 0 {
			return fmt.Errorf("--db --table and at least one --set col=val are required")
		}
		body, err := apiRequest("POST", fmt.Sprintf("/admin/databases/%s/tables/%s/rows", database, table), map[string]any{"values": parseKV(set)})
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	rowsInsert.Flags().String("db", "", "database")
	rowsInsert.Flags().String("table", "", "table")
	rowsInsert.Flags().StringArray("set", nil, "col=val (repeatable)")
	rows.AddCommand(rowsInsert)

	rowsUpdate := &cobra.Command{Use: "update", Short: "Update rows (--set … --where …; critical)", RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := reqStr(cmd, "db")
		table, _ := cmd.Flags().GetString("table")
		set, _ := cmd.Flags().GetStringArray("set")
		where, _ := cmd.Flags().GetStringArray("where")
		if database == "" || table == "" || len(set) == 0 || len(where) == 0 {
			return fmt.Errorf("--db --table --set --where are required (where prevents mass updates)")
		}
		if err := mustConfirm(fmt.Sprintf("update rows in %s.%s where %v", database, table, where)); err != nil {
			return err
		}
		body, err := apiRequest("PATCH", fmt.Sprintf("/admin/databases/%s/tables/%s/rows", database, table), map[string]any{"set": parseKV(set), "where": parseKV(where)})
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	rowsUpdate.Flags().String("db", "", "database")
	rowsUpdate.Flags().String("table", "", "table")
	rowsUpdate.Flags().StringArray("set", nil, "col=val (repeatable)")
	rowsUpdate.Flags().StringArray("where", nil, "col=val match (repeatable)")
	rows.AddCommand(rowsUpdate)

	rowsDelete := &cobra.Command{Use: "delete", Short: "Delete rows (--where …; critical)", RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := reqStr(cmd, "db")
		table, _ := cmd.Flags().GetString("table")
		where, _ := cmd.Flags().GetStringArray("where")
		if database == "" || table == "" || len(where) == 0 {
			return fmt.Errorf("--db --table --where are required (where prevents mass deletes)")
		}
		if err := mustConfirm(fmt.Sprintf("DELETE rows in %s.%s where %v", database, table, where)); err != nil {
			return err
		}
		body, err := apiRequest("DELETE", fmt.Sprintf("/admin/databases/%s/tables/%s/rows", database, table), map[string]any{"where": parseKV(where)})
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	rowsDelete.Flags().String("db", "", "database")
	rowsDelete.Flags().String("table", "", "table")
	rowsDelete.Flags().StringArray("where", nil, "col=val match (repeatable)")
	rows.AddCommand(rowsDelete)

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

// fetchRows runs a SELECT via /admin/execute and returns the rows as maps.
func fetchRows(database, query string) []map[string]any {
	body, err := apiRequest("POST", "/admin/execute", map[string]any{"query": query, "database": database})
	if err != nil {
		return nil
	}
	raw, _ := body["rows"].([]any)
	if raw == nil {
		if data, ok := body["data"].(map[string]any); ok {
			raw, _ = data["rows"].([]any)
		}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// tableComments returns {table name: comment} for the public schema (obj_description).
func tableComments(database string) map[string]string {
	q := `SELECT c.relname AS name, obj_description(c.oid) AS comment
	      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
	      WHERE c.relkind IN ('r','p','v','m') AND n.nspname = 'public'`
	m := map[string]string{}
	for _, row := range fetchRows(database, q) {
		name, _ := row["name"].(string)
		if cmt, ok := row["comment"].(string); ok && name != "" && cmt != "" {
			m[name] = cmt
		}
	}
	return m
}

// columnComments returns {column name: comment} for one table (col_description).
func columnComments(database, table string) map[string]string {
	esc := strings.ReplaceAll(table, "'", "''")
	q := fmt.Sprintf(`SELECT a.attname AS name, col_description(a.attrelid, a.attnum) AS comment
	      FROM pg_attribute a
	      WHERE a.attrelid = ('public.' || quote_ident('%s'))::regclass
	        AND a.attnum > 0 AND NOT a.attisdropped`, esc)
	m := map[string]string{}
	for _, row := range fetchRows(database, q) {
		name, _ := row["name"].(string)
		if cmt, ok := row["comment"].(string); ok && name != "" && cmt != "" {
			m[name] = cmt
		}
	}
	return m
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

// parseKV turns ["col=val", "c2=v2"] into {col:val, c2:v2} (value = everything after first '=').
func parseKV(pairs []string) map[string]any {
	m := map[string]any{}
	for _, p := range pairs {
		i := strings.IndexByte(p, '=')
		if i < 0 {
			continue
		}
		m[strings.TrimSpace(p[:i])] = p[i+1:]
	}
	return m
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func featureCommands() []*cobra.Command {
	return []*cobra.Command{endpointCmds(), webhookCmds(), embedCmds(), tokenCmds(), userCmds(), infraCmds()}
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---- endpoints ----
func endpointCmds() *cobra.Command {
	c := &cobra.Command{Use: "endpoints", Short: "Manage API endpoints"}
	list := &cobra.Command{Use: "list", Short: "List endpoints", RunE: func(cmd *cobra.Command, a []string) error {
		database, err := reqStr(cmd, "db")
		if err != nil {
			return err
		}
		q := ""
		if t, _ := cmd.Flags().GetString("table"); t != "" {
			q = "?table=" + t
		}
		body, err := apiRequest("GET", "/admin/databases/"+database+"/endpoints"+q, nil)
		if err != nil {
			return err
		}
		printJSON(body["endpoints"])
		return nil
	}}
	list.Flags().String("db", "", "database")
	list.Flags().String("table", "", "filter by table")
	c.AddCommand(list)

	create := &cobra.Command{Use: "create", Short: "Create an endpoint (read or vector). Public = critical.", RunE: func(cmd *cobra.Command, a []string) error {
		database, err := reqStr(cmd, "db")
		if err != nil {
			return err
		}
		var payload map[string]any
		if raw, _ := cmd.Flags().GetString("json"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				return fmt.Errorf("invalid --json: %v", err)
			}
		} else {
			name, _ := cmd.Flags().GetString("name")
			table, _ := cmd.Flags().GetString("table")
			cols, _ := cmd.Flags().GetString("columns")
			if name == "" || table == "" || cols == "" {
				return fmt.Errorf("--name --table --columns are required (or pass --json)")
			}
			etype, _ := cmd.Flags().GetString("type")
			access, _ := cmd.Flags().GetString("access")
			token, _ := cmd.Flags().GetString("token")
			columns := splitCSV(cols)
			var config map[string]any
			if etype == "vector" {
				vcol, _ := cmd.Flags().GetString("vector-column")
				provider, _ := cmd.Flags().GetString("provider")
				model, _ := cmd.Flags().GetString("model")
				if vcol == "" || provider == "" || model == "" {
					return fmt.Errorf("vector endpoints need --vector-column --provider --model")
				}
				config = map[string]any{"columns": columns, "vectorColumn": vcol, "provider": provider, "model": model, "pagination": map[string]any{"limit": 10}}
			} else {
				etype = "read"
				var sort any
				if s, _ := cmd.Flags().GetString("sort"); s != "" {
					parts := strings.SplitN(s, ":", 2)
					dir := "asc"
					if len(parts) == 2 {
						dir = parts[1]
					}
					sort = map[string]any{"column": parts[0], "direction": dir}
				}
				filt, _ := cmd.Flags().GetString("filterable")
				config = map[string]any{"columns": columns, "filterableColumns": splitCSV(filt), "sort": sort, "pagination": map[string]any{"limit": 20}, "format": "json"}
			}
			payload = map[string]any{"name": name, "table": table, "type": etype, "config": config, "access": access}
			if token != "" {
				payload["authTokenId"] = token
			}
			if p, _ := cmd.Flags().GetString("path"); p != "" {
				payload["path"] = p
			}
		}
		if payload["access"] == "public" {
			if err := mustConfirm("publish a PUBLIC endpoint on " + database); err != nil {
				return err
			}
		}
		body, err := apiRequest("POST", "/admin/databases/"+database+"/endpoints", payload)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	create.Flags().String("db", "", "database")
	create.Flags().String("name", "", "endpoint name")
	create.Flags().String("path", "", "URL path (default: from name)")
	create.Flags().String("table", "", "table")
	create.Flags().String("type", "read", "read | vector")
	create.Flags().String("columns", "", "comma list of columns to return")
	create.Flags().String("filterable", "", "read: comma list of filterable columns")
	create.Flags().String("sort", "", "read: sort as col:asc|desc")
	create.Flags().String("vector-column", "", "vector: embedding column to search")
	create.Flags().String("provider", "", "vector: query embedder provider")
	create.Flags().String("model", "", "vector: query embedder model")
	create.Flags().String("access", "protected", "protected | public")
	create.Flags().String("token", "", "auth token id (protected)")
	create.Flags().String("json", "", "raw endpoint body (overrides flags)")
	c.AddCommand(create)

	del := &cobra.Command{Use: "delete", Short: "Delete an endpoint (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if err := mustConfirm("delete endpoint " + id); err != nil {
			return err
		}
		if _, err := apiRequest("DELETE", "/admin/endpoints/"+id, nil); err != nil {
			return err
		}
		fmt.Println("✓ Deleted endpoint", id)
		return nil
	}}
	del.Flags().String("id", "", "endpoint id")
	c.AddCommand(del)
	return c
}

// ---- webhooks ----
func webhookCmds() *cobra.Command {
	c := &cobra.Command{Use: "webhooks", Short: "Manage webhooks"}
	list := &cobra.Command{Use: "list", Short: "List webhooks", RunE: func(cmd *cobra.Command, a []string) error {
		database, err := reqStr(cmd, "db")
		if err != nil {
			return err
		}
		q := ""
		if t, _ := cmd.Flags().GetString("table"); t != "" {
			q = "?table=" + t
		}
		body, err := apiRequest("GET", "/admin/databases/"+database+"/webhooks"+q, nil)
		if err != nil {
			return err
		}
		printJSON(body["webhooks"])
		return nil
	}}
	list.Flags().String("db", "", "database")
	list.Flags().String("table", "", "filter by table")
	c.AddCommand(list)

	create := &cobra.Command{Use: "create", Short: "Create a webhook (critical — outbound to a URL)", RunE: func(cmd *cobra.Command, a []string) error {
		database, _ := reqStr(cmd, "db")
		table, _ := cmd.Flags().GetString("table")
		name, _ := cmd.Flags().GetString("name")
		url, _ := cmd.Flags().GetString("url")
		events, _ := cmd.Flags().GetString("events")
		method, _ := cmd.Flags().GetString("method")
		if database == "" || table == "" || name == "" || url == "" || events == "" {
			return fmt.Errorf("--db --table --name --url --events are required")
		}
		if err := mustConfirm("create webhook to external URL " + url); err != nil {
			return err
		}
		payload := map[string]any{"name": name, "table": table, "url": url, "method": method, "events": strings.Split(events, ",")}
		body, err := apiRequest("POST", "/admin/databases/"+database+"/webhooks", payload)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	create.Flags().String("db", "", "database")
	create.Flags().String("table", "", "table")
	create.Flags().String("name", "", "webhook name")
	create.Flags().String("url", "", "target URL")
	create.Flags().String("events", "insert", "comma list: insert,update,delete")
	create.Flags().String("method", "POST", "HTTP method")
	c.AddCommand(create)

	test := &cobra.Command{Use: "test", Short: "Send a sample payload", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		body, err := apiRequest("POST", "/admin/webhooks/"+id+"/test", nil)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	test.Flags().String("id", "", "webhook id")
	c.AddCommand(test)

	del := &cobra.Command{Use: "delete", Short: "Delete a webhook (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if err := mustConfirm("delete webhook " + id); err != nil {
			return err
		}
		if _, err := apiRequest("DELETE", "/admin/webhooks/"+id, nil); err != nil {
			return err
		}
		fmt.Println("✓ Deleted webhook", id)
		return nil
	}}
	del.Flags().String("id", "", "webhook id")
	c.AddCommand(del)
	return c
}

// ---- embeddings ----
func embedCmds() *cobra.Command {
	c := &cobra.Command{Use: "embed", Short: "Embedding generation"}
	providers := &cobra.Command{Use: "providers", Short: "List embedding providers", RunE: func(cmd *cobra.Command, a []string) error {
		body, err := apiRequest("GET", "/admin/embedding/providers", nil)
		if err != nil {
			return err
		}
		printJSON(body["providers"])
		return nil
	}}
	c.AddCommand(providers)

	run := &cobra.Command{Use: "run", Short: "Embed a table column (single-column mode)", RunE: func(cmd *cobra.Command, a []string) error {
		database, _ := reqStr(cmd, "db")
		table, _ := cmd.Flags().GetString("table")
		template, _ := cmd.Flags().GetString("template")
		provider, _ := cmd.Flags().GetString("provider")
		model, _ := cmd.Flags().GetString("model")
		column, _ := cmd.Flags().GetString("column")
		if database == "" || table == "" || template == "" || provider == "" || model == "" {
			return fmt.Errorf("--db --table --template --provider --model are required")
		}
		if column == "" {
			column = table + "_embedding"
		}
		payload := map[string]any{
			"table": table, "template": template, "provider": provider, "model": model,
			"chunking": map[string]any{"strategy": "whole"},
			"storage":  map[string]any{"mode": "column", "column": column},
		}
		body, err := apiRequest("POST", "/admin/databases/"+database+"/embeddings/run", payload)
		if err != nil {
			return err
		}
		jobID, _ := body["jobId"].(string)
		if jobID == "" {
			printJSON(body)
			return nil
		}
		fmt.Printf("job %s running…\n", jobID)
		for {
			time.Sleep(1500 * time.Millisecond)
			j, err := apiRequest("GET", "/admin/embedding/jobs/"+jobID, nil)
			if err != nil {
				return err
			}
			st, _ := j["status"].(string)
			fmt.Printf("  %v/%v (%s)\n", j["processed"], j["total"], st)
			if st == "done" {
				fmt.Printf("✓ Done — %v vectors (%v-dim)\n", j["processed"], j["dimensions"])
				return nil
			}
			if st == "error" {
				return fmt.Errorf("job failed: %v", j["error"])
			}
		}
	}}
	run.Flags().String("db", "", "database")
	run.Flags().String("table", "", "table")
	run.Flags().String("template", "", "text template, e.g. {{summary}}")
	run.Flags().String("provider", "ollama", "ollama|openai|gemini")
	run.Flags().String("model", "", "model name")
	run.Flags().String("column", "", "target vector column (default <table>_embedding)")
	c.AddCommand(run)

	// embed key set/remove (cloud provider API keys)
	key := &cobra.Command{Use: "key", Short: "Manage cloud provider API keys"}
	keySet := &cobra.Command{Use: "set", Short: "Set a provider API key (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		provider, _ := reqStr(cmd, "provider")
		val, _ := cmd.Flags().GetString("key")
		if provider == "" || val == "" {
			return fmt.Errorf("--provider and --key are required")
		}
		if err := mustConfirm("set API key for " + provider); err != nil {
			return err
		}
		if _, err := apiRequest("PUT", "/admin/embedding/providers/"+provider+"/key", map[string]any{"key": val}); err != nil {
			return err
		}
		fmt.Printf("✓ Key set for %s\n", provider)
		return nil
	}}
	keySet.Flags().String("provider", "", "openai | gemini")
	keySet.Flags().String("key", "", "API key")
	key.AddCommand(keySet)
	keyRm := &cobra.Command{Use: "remove", Short: "Remove a provider API key", RunE: func(cmd *cobra.Command, a []string) error {
		provider, _ := reqStr(cmd, "provider")
		if provider == "" {
			return fmt.Errorf("--provider is required")
		}
		if _, err := apiRequest("DELETE", "/admin/embedding/providers/"+provider+"/key", nil); err != nil {
			return err
		}
		fmt.Printf("✓ Key removed for %s\n", provider)
		return nil
	}}
	keyRm.Flags().String("provider", "", "openai | gemini")
	key.AddCommand(keyRm)
	c.AddCommand(key)

	// embed auto (incremental re-embed configs)
	auto := &cobra.Command{Use: "auto", Short: "Manage auto re-embed (keep-in-sync) configs"}
	autoList := &cobra.Command{Use: "list", Short: "List auto-embed configs", RunE: func(cmd *cobra.Command, a []string) error {
		database, err := reqStr(cmd, "db")
		if err != nil {
			return err
		}
		body, err := apiRequest("GET", "/admin/databases/"+database+"/auto-embed", nil)
		if err != nil {
			return err
		}
		printJSON(body["configs"])
		return nil
	}}
	autoList.Flags().String("db", "", "database")
	auto.AddCommand(autoList)
	autoDel := &cobra.Command{Use: "delete", Short: "Delete an auto-embed config", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if _, err := apiRequest("DELETE", "/admin/auto-embed/"+id, nil); err != nil {
			return err
		}
		fmt.Println("✓ Deleted auto-embed config", id)
		return nil
	}}
	autoDel.Flags().String("id", "", "config id")
	auto.AddCommand(autoDel)
	c.AddCommand(auto)
	return c
}

// ---- tokens ----
func tokenCmds() *cobra.Command {
	c := &cobra.Command{Use: "tokens", Short: "Manage API tokens"}
	list := &cobra.Command{Use: "list", Short: "List tokens", RunE: func(cmd *cobra.Command, a []string) error {
		raw, code, err := apiRequestRaw("GET", "/api/tokens", nil)
		if err != nil {
			return err
		}
		if code >= 400 {
			return fmt.Errorf("list failed (%d)", code)
		}
		fmt.Println(string(raw))
		return nil
	}}
	c.AddCommand(list)

	create := &cobra.Command{Use: "create", Short: "Create an API token (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		name, _ := cmd.Flags().GetString("name")
		dbs, _ := cmd.Flags().GetStringSlice("db")
		perms, _ := cmd.Flags().GetString("perms")
		if name == "" || len(dbs) == 0 {
			return fmt.Errorf("--name and at least one --db are required")
		}
		if err := mustConfirm("create API token " + name); err != nil {
			return err
		}
		permList := strings.Split(perms, ",")
		permMap := map[string]any{}
		for _, d := range dbs {
			permMap[d] = permList
		}
		body, err := apiRequest("POST", "/api/tokens", map[string]any{"tokenName": name, "databases": dbs, "permissions": permMap})
		if err != nil {
			return err
		}
		printJSON(body) // includes the secret token once
		return nil
	}}
	create.Flags().String("name", "", "token name")
	create.Flags().StringSlice("db", nil, "database(s) the token can access")
	create.Flags().String("perms", "read", "comma perms: read,write,admin")
	c.AddCommand(create)

	del := &cobra.Command{Use: "delete", Short: "Revoke a token (critical)", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if err := mustConfirm("revoke token " + id); err != nil {
			return err
		}
		if _, _, err := apiRequestRaw("DELETE", "/api/tokens/"+id, nil); err != nil {
			return err
		}
		fmt.Println("✓ Revoked token", id)
		return nil
	}}
	del.Flags().String("id", "", "token id")
	c.AddCommand(del)

	reveal := &cobra.Command{Use: "reveal", Short: "Reveal a token's secret (admin; critical)", RunE: func(cmd *cobra.Command, a []string) error {
		id, err := reqStr(cmd, "id")
		if err != nil {
			return err
		}
		if err := mustConfirm("reveal secret for token " + id); err != nil {
			return err
		}
		body, err := apiRequest("POST", "/api/tokens/"+id+"/reveal", nil)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}}
	reveal.Flags().String("id", "", "token id")
	c.AddCommand(reveal)
	return c
}

// ---- infra / local inference ----
func infraCmds() *cobra.Command {
	c := &cobra.Command{Use: "infra", Short: "Instance infrastructure"}
	li := &cobra.Command{Use: "local-inference", Short: "Local inference (boiler-connect) status/pairing"}
	li.AddCommand(&cobra.Command{Use: "status", Short: "Show local inference status", RunE: func(cmd *cobra.Command, a []string) error {
		body, err := apiRequest("GET", "/admin/local-inference/status", nil)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}})
	li.AddCommand(&cobra.Command{Use: "pair", Short: "Get a pairing command for boiler-connect", RunE: func(cmd *cobra.Command, a []string) error {
		body, err := apiRequest("POST", "/admin/local-inference/pair", nil)
		if err != nil {
			return err
		}
		printJSON(body)
		return nil
	}})
	c.AddCommand(li)
	return c
}

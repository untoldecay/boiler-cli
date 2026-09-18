// boiler — admin control-plane CLI for Boiler.
//
// Lets an admin (or an admin agent) drive Boiler from the terminal: databases,
// tables, rows, endpoints, webhooks, embeddings, tokens, local inference.
// Auth: `boiler login` → short-lived JWT (auto-refreshed). Critical/destructive
// actions require --confirm (and the human approves the command in the client).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "boiler",
		Short:         "Admin control-plane CLI for Boiler",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&confirmFlag, "confirm", false, "confirm a critical/destructive action")

	root.AddCommand(authCommands()...)
	root.AddCommand(dataCommands()...)
	root.AddCommand(featureCommands()...)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "✗ "+err.Error())
		os.Exit(1)
	}
}

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
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build metadata, injected via -ldflags "-X main.version=… -X main.commit=… -X main.date=…".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// effectiveVersion prefers the ldflags value; otherwise falls back to the module
// version from build info, so `go install …@vX.Y.Z` reports the tag with no ldflags.
func effectiveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	ver := effectiveVersion()
	root := &cobra.Command{
		Use:           "boiler",
		Short:         "Admin control-plane CLI for Boiler",
		Version:       ver,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// `boiler --version` / `-v` (cobra auto-registers -v since the shorthand is free).
	root.SetVersionTemplate(fmt.Sprintf("boiler {{.Version}} (commit %s, built %s)\n", commit, date))
	root.PersistentFlags().BoolVar(&confirmFlag, "confirm", false, "confirm a critical/destructive action")

	// `boiler version` subcommand — same info, for muscle memory.
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show the boiler CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("boiler %s (commit %s, built %s)\n", ver, commit, date)
			return nil
		},
	})

	root.AddCommand(authCommands()...)
	root.AddCommand(dataCommands()...)
	root.AddCommand(featureCommands()...)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "✗ "+err.Error())
		os.Exit(1)
	}
}

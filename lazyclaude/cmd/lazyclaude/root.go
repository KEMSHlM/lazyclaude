package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lazyclaude",
		Short:   "A standalone TUI for Claude Code",
		Long:    "lazyclaude is a terminal UI for managing Claude Code sessions, inspired by lazygit.",
		Version: fmt.Sprintf("%s (%s)", version, commit),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: launch main TUI
			fmt.Println("lazyclaude TUI - not yet implemented")
			return nil
		},
	}

	cmd.AddCommand(newServerCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newToolCmd())
	cmd.AddCommand(newSetupCmd())

	return cmd
}
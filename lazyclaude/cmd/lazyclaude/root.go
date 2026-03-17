package main

import (
	"fmt"

	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lazyclaude",
		Short:   "A standalone TUI for Claude Code",
		Long:    "lazyclaude is a terminal UI for managing Claude Code sessions, inspired by lazygit.",
		Version: fmt.Sprintf("%s (%s)", version, commit),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := gui.NewApp(gui.ModeMain)
			if err != nil {
				return fmt.Errorf("init TUI: %w", err)
			}
			return app.Run()
		},
	}

	cmd.AddCommand(newServerCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newToolCmd())
	cmd.AddCommand(newSetupCmd())

	return cmd
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Register tmux keybindings and hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("setup - not yet implemented")
			return nil
		},
	}
}
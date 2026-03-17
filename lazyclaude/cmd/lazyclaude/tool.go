package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newToolCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tool",
		Short: "Show tool confirmation popup",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("tool popup - not yet implemented")
			return nil
		},
	}
}
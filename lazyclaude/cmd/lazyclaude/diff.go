package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var window, oldFile, newFile string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show diff popup viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("diff popup - window=%s old=%s new=%s\n", window, oldFile, newFile)
			fmt.Println("diff popup - not yet implemented")
			return nil
		},
	}

	cmd.Flags().StringVar(&window, "window", "", "tmux window name")
	cmd.Flags().StringVar(&oldFile, "old", "", "old file path")
	cmd.Flags().StringVar(&newFile, "new", "", "new file contents path")

	return cmd
}

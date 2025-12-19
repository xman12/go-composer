package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	workDir string
)

var rootCmd = &cobra.Command{
	Use:   "go-composer",
	Short: "A fast PHP dependency manager written in Go",
	Long: `go-composer is a high-performance alternative to Composer,
written in Go for speed and efficiency.

It reads composer.json, resolves dependencies from Packagist,
and installs packages into the vendor/ directory.

Original source: https://github.com/xman12/go-composer
Copyright (c) 2025 Aleksandr Belyshev`,
	Version: "1.0.0",
}

// Execute выполняет root команду
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&workDir, "working-dir", "d", ".", "working directory")
}


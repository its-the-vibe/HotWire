package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	setApp  string
	setFile string
)

// configCmd groups configuration-related sub-commands.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage application configuration",
}

// configSetCmd implements: hotwire config set --app <name> --file <path>
var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Upload a JSON config file and signal connected clients to reload",
	RunE:  runConfigSet,
}

func init() {
	configSetCmd.Flags().StringVar(&setApp, "app", "", "Application name (required)")
	configSetCmd.Flags().StringVar(&setFile, "file", "", "Path to the JSON configuration file (required)")

	_ = configSetCmd.MarkFlagRequired("app")
	_ = configSetCmd.MarkFlagRequired("file")

	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, _ []string) error {
	raw, err := os.ReadFile(setFile)
	if err != nil {
		return fmt.Errorf("reading file %q: %w", setFile, err)
	}

	if !json.Valid(raw) {
		return fmt.Errorf("file %q does not contain valid JSON", setFile)
	}

	ctx := context.Background()
	rdb := newRedisClient()
	defer rdb.Close()

	key := fmt.Sprintf("config:%s", setApp)
	channel := fmt.Sprintf("ch:%s", setApp)

	if err := rdb.Set(ctx, key, raw, 0).Err(); err != nil {
		return fmt.Errorf("writing config to Redis key %q: %w", key, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Config written to key %q\n", key)

	if err := rdb.Publish(ctx, channel, "reload").Err(); err != nil {
		return fmt.Errorf("publishing reload signal to channel %q: %w", channel, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Reload signal published to channel %q\n", channel)

	return nil
}

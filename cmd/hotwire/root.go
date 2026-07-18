package main

import (
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	redisAddr     string
	redisPassword string
	redisDB       int
)

var rootCmd = &cobra.Command{
	Use:   "hotwire",
	Short: "HotWire – real-time configuration hot-reload CLI",
	Long: `HotWire is an administrative CLI for the HotWire real-time configuration
service. It validates, uploads, and publishes configuration changes to
distributed clients without requiring application restarts.`,
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&redisAddr, "redis-addr", "localhost:6379", "Redis server address (host:port)")
	rootCmd.PersistentFlags().StringVar(&redisPassword, "redis-password", "", "Redis password")
	rootCmd.PersistentFlags().IntVar(&redisDB, "redis-db", 0, "Redis database index")
}

// newRedisClient constructs a redis.Client from the persistent root flags.
func newRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
}

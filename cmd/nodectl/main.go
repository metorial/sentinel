package main

import (
	"fmt"
	"os"

	"github.com/metorial/sentinel/internal/cli"
	"github.com/spf13/cobra"
)

var (
	serverURL  string
	outputJSON bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "nodectl",
	Short: "CLI for Node Metrics Collector",
	Long: `nodectl is a command-line interface for interacting with the Node Metrics Collector API.

It provides commands to query host information, usage statistics, and cluster-wide metrics.`,
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check collector service health",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cli.NewClient(serverURL)
		data, err := client.Health()
		if err != nil {
			return err
		}

		if outputJSON {
			return cli.FormatJSON(data)
		}

		status := data["status"].(string)
		database := data["database"].(string)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Database: %s\n", database)
		return nil
	},
}

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage and query hosts",
}

var listHostsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cli.NewClient(serverURL)
		data, err := client.ListHosts()
		if err != nil {
			return err
		}

		if outputJSON {
			return cli.FormatJSON(data)
		}

		return cli.FormatHostsTable(data)
	},
}

var getHostCmd = &cobra.Command{
	Use:   "get [hostname]",
	Short: "Get detailed information about a specific host",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		client := cli.NewClient(serverURL)
		data, err := client.GetHost(hostname, limit)
		if err != nil {
			return err
		}

		if outputJSON {
			return cli.FormatJSON(data)
		}

		return cli.FormatHostDetailTable(data)
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get cluster-wide statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cli.NewClient(serverURL)
		data, err := client.GetStats()
		if err != nil {
			return err
		}

		if outputJSON {
			return cli.FormatJSON(data)
		}

		return cli.FormatStatsTable(data)
	},
}

func init() {
	// Check for environment variable, fallback to default
	defaultServerURL := os.Getenv("CONTROLLER_URL")
	if defaultServerURL == "" {
		defaultServerURL = "http://localhost:8080"
	}

	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", defaultServerURL, "Collector server URL")
	rootCmd.PersistentFlags().BoolVarP(&outputJSON, "json", "j", false, "Output in JSON format")

	getHostCmd.Flags().IntP("limit", "l", 100, "Number of usage records to retrieve (max: 1000)")

	hostsCmd.AddCommand(listHostsCmd)
	hostsCmd.AddCommand(getHostCmd)

	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(hostsCmd)
	rootCmd.AddCommand(statsCmd)
}

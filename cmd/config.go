package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/simonyos/Z-CODE/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage z-code configuration",
	Long: `Manage z-code configuration including API keys and defaults.

Examples:
  zcode config                      # Show current config
  zcode config set openai <key>     # Set OpenAI API key
  zcode config set provider openai  # Set default provider
  zcode config delete openai        # Remove OpenAI API key`,
	Run: func(cmd *cobra.Command, args []string) {
		showConfig()
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Available keys:
  openai       - OpenAI API key
  anthropic    - Anthropic API key
  openrouter   - OpenRouter API key
  litellm      - LiteLLM API key
  litellm_url  - LiteLLM base URL (default: http://localhost:4000)
  provider     - Default provider (claude, openai, openrouter, litellm)
  model        - Default model`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		if err := config.Set(key, value); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Set %s successfully.\n", key)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		keys := config.ListKeys()

		if val, ok := keys[key]; ok {
			fmt.Printf("%s: %s\n", key, val)
		} else {
			fmt.Printf("%s is not set\n", key)
		}
	},
}

var configDeleteCmd = &cobra.Command{
	Use:     "delete <key>",
	Aliases: []string{"remove", "unset"},
	Short:   "Delete a configuration value",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		if err := config.Delete(key); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Deleted %s.\n", key)
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.ConfigPath())
	},
}

func showConfig() {
	fmt.Printf("Configuration file: %s\n\n", config.ConfigPath())

	keys := config.ListKeys()
	if len(keys) == 0 {
		fmt.Println("No configuration set.")
		fmt.Println("\nUse 'zcode config set <key> <value>' to configure.")
		return
	}

	for k, v := range keys {
		fmt.Printf("  %s: %s\n", k, v)
	}
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configDeleteCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd)
}

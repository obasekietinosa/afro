package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new bundle",
	Long:  `Initialize a new bundle by creating an afro.yaml configuration file in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Initializing new Afro bundle...")

	// Base URL
	fmt.Print("Base URL: ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	viper.Set("base_url", baseURL)

	// Authentication
	fmt.Print("Configure Basic Authentication? [y/N]: ")
	authConfirm, _ := reader.ReadString('\n')
	authConfirm = strings.TrimSpace(strings.ToLower(authConfirm))

	if authConfirm == "y" || authConfirm == "yes" {
		fmt.Print("Username: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)

		fmt.Print("Password: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		viper.Set("auth.username", username)
		viper.Set("auth.password", password)
	}

	// Common Headers
	fmt.Print("Configure common headers? [y/N]: ")
	headersConfirm, _ := reader.ReadString('\n')
	headersConfirm = strings.TrimSpace(strings.ToLower(headersConfirm))

	if headersConfirm == "y" || headersConfirm == "yes" {
		headers := make(map[string]string)
		for {
			fmt.Print("Header Name (leave empty to finish): ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				break
			}

			fmt.Print("Header Value: ")
			value, _ := reader.ReadString('\n')
			value = strings.TrimSpace(value)

			headers[name] = value
		}
		viper.Set("headers", headers)
	}

	// Save config
	filename := "afro.yaml"
	bundle, _ := rootCmd.Flags().GetString("bundle")
	if bundle != "" {
		filename = bundle + ".yaml"
	}

	err := viper.WriteConfigAs(filename)
	if err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Bundle initialized! Configuration saved to %s\n", filename)
}

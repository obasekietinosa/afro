package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "afro",
	Short: "Afro is an API Client whose core philosophy is chaining requests together.",
	Long: `Afro lets you make HTTP requests from the command line and save the request setup
as well replay it or use its response as part of another.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./afro.yaml)")
	rootCmd.PersistentFlags().String("bundle", "", "specify what bundle to use for the command")
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in current directory with name "afro" (without extension).
		viper.AddConfigPath(".")

		// Check if bundle flag is set
		bundle, _ := rootCmd.Flags().GetString("bundle")
		if bundle != "" {
			viper.SetConfigName(bundle)
		} else {
			viper.SetConfigName("afro")
		}
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	viper.ReadInConfig()
}

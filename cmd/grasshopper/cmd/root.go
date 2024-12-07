/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

var config = &Config{}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grasshopper",
	Short: "A secure UDP relayer",
	Long: `The grasshopper will listen for incoming UDP packets and forward them to the configured destination.
Optionally, the listener can be configured to apply cryptogrraphy on both the incoming and outgoing packets, with different keys and methods.
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentPersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cmd.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.

	rootCmd.PersistentFlags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringVarP(&config.Listen, "listen", "l", ":1234", "listener address, eg: \"IP:1234\"")
	rootCmd.PersistentFlags().IntVar(&config.SockBuf, "sockbuf", 1024*1024, "socket buffer for listener")
	rootCmd.PersistentFlags().StringVarP(&config.NextHop, "nexthop", "n", "127.0.0.1:3000", "the server to forward to")
	rootCmd.PersistentFlags().StringVar(&config.KI, "ki", "it's a secret", "The secret to encrypt and decrypt for the last hop(incoming)")
	rootCmd.PersistentFlags().StringVar(&config.KO, "ko", "it's a secret", "The secret to encrypt and decrypt for the next hop(outgoing)")
	rootCmd.PersistentFlags().StringVar(&config.CI, "ci", "aes", "The crytpgraphy method for incoming data, available: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none")
	rootCmd.PersistentFlags().StringVar(&config.CO, "co", "aes", "The crytpgraphy method for outgoing data, available: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none")
	rootCmd.PersistentFlags().DurationVar(&config.Timeout, "timeout", 600*time.Second, "set how long an UDP connection can live when in idle(in seconds)")
}

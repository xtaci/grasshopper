/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"crypto/sha1"
	"log"

	"github.com/spf13/cobra"
	"github.com/xtaci/grasshopper"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// SALT is use for pbkdf2 key expansion
	SALT = "GRASSHOPPER"
)

var VERSION = "undefined"

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a listener for UDP packet forwarding",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("version:", VERSION)
		log.Println("listening on:", config.Listen)
		log.Println("next hop:", config.NextHop)
		log.Println("socket buffer:", config.SockBuf)
		log.Println("incoming crypto:", config.CI)
		log.Println("outgoing crypto:", config.CO)

		log.Println("initiating key derivation KI")
		passIn := pbkdf2.Key([]byte(config.KI), []byte(SALT), 4096, 32, sha1.New)
		log.Println("initiating key derivation KO")
		passOut := pbkdf2.Key([]byte(config.KO), []byte(SALT), 4096, 32, sha1.New)
		log.Println("key derivation done")

		// init crypter
		crypterIn := newCrypt(passIn, config.CI)
		crypterOut := newCrypt(passOut, config.CO)

		log.Println(crypterIn, crypterOut)

		// init listener
		listener, err := grasshopper.ListenWithOptions(config.Listen, config.NextHop, config.SockBuf, config.Timeout, crypterIn, crypterOut, log.Default())
		if err != nil {
			log.Fatal(err)
		}

		listener.Start()
	},
}

func newCrypt(pass []byte, method string) grasshopper.BlockCrypt {
	var block grasshopper.BlockCrypt
	switch method {
	case "none":
		block = nil
	case "sm4":
		block, _ = grasshopper.NewSM4BlockCrypt(pass[:16])
	case "tea":
		block, _ = grasshopper.NewTEABlockCrypt(pass[:16])
	case "aes":
		block, _ = grasshopper.NewAESBlockCrypt(pass)
	case "aes-128":
		block, _ = grasshopper.NewAESBlockCrypt(pass[:16])
	case "aes-192":
		block, _ = grasshopper.NewAESBlockCrypt(pass[:24])
	case "blowfish":
		block, _ = grasshopper.NewBlowfishBlockCrypt(pass)
	case "twofish":
		block, _ = grasshopper.NewTwofishBlockCrypt(pass)
	case "cast5":
		block, _ = grasshopper.NewCast5BlockCrypt(pass[:16])
	case "3des":
		block, _ = grasshopper.NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		block, _ = grasshopper.NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		block, _ = grasshopper.NewSalsa20BlockCrypt(pass)
	}
	return block
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

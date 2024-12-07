package main

import (
	"crypto/sha1"
	"log"
	"os"
	"time"

	"github.com/urfave/cli"
	"github.com/xtaci/udpcast"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// SALT is use for pbkdf2 key expansion
	SALT = "UDPCAST"
)

var VERSION = "undefined"

var (
	// sharedOptions is the flags shared by both client and server
	sharedOptions = []cli.Flag{
		cli.StringFlag{
			Name:  "listen,l",
			Value: ":1234",
			Usage: `server listen address, eg: "IP:1234" for a single port`,
		},
		cli.StringFlag{
			Name:   "key",
			Value:  "it's a secret",
			Usage:  "pre-shared secret between client and server",
			EnvVar: "UDPCAST_KEY",
		},
		cli.StringFlag{
			Name:  "crypt",
			Value: "aes",
			Usage: "aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: 600 * time.Second,
			Usage: "set how long an UDP connection can live when in idle(in seconds)",
		},
		cli.IntFlag{
			Name:  "sockbuf",
			Value: 1024 * 1024, // socket buffer size in bytes
			Usage: "per-socket buffer in bytes",
		},
		cli.StringFlag{
			Name:  "c",
			Value: "", // when the value is not empty, the config path must exists
			Usage: "config from json file, which will override the command from shell",
		},
	}
)

func main() {
	myApp := cli.NewApp()
	myApp.Name = "udpcast"
	myApp.Commands = []cli.Command{
		{
			Name:    "server",
			Aliases: []string{"s"},
			Usage:   "start in server mode",
			Flags: append([]cli.Flag{
				cli.StringFlag{
					Name:  "target, t",
					Value: "127.0.0.1:3000",
					Usage: "target server address",
				},
			}, sharedOptions...),
			Action: run,
		},
	}

	err := myApp.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	config := Config{}
	config.Listen = c.String("listen")
	config.Target = c.String("target")
	config.Key = c.String("key")
	config.Crypt = c.String("crypt")
	config.Mode = c.String("mode")
	config.Timeout = c.Duration("timeout")
	config.SockBuf = c.Int("sockbuf")

	log.Println("version:", VERSION)
	log.Println("listening on:", config.Listen)
	log.Println("mode:", c.Command.Name)
	log.Println("socket buffer:", config.SockBuf)
	log.Println("encryption:", config.Crypt)
	log.Println("initiating key derivation")
	pass := pbkdf2.Key([]byte(config.Key), []byte(SALT), 4096, 32, sha1.New)
	log.Println("key derivation done")
	var block udpcast.BlockCrypt
	switch config.Crypt {
	case "none":
		block = nil
	case "sm4":
		block, _ = udpcast.NewSM4BlockCrypt(pass[:16])
	case "tea":
		block, _ = udpcast.NewTEABlockCrypt(pass[:16])
	case "aes-128":
		block, _ = udpcast.NewAESBlockCrypt(pass[:16])
	case "aes-192":
		block, _ = udpcast.NewAESBlockCrypt(pass[:24])
	case "blowfish":
		block, _ = udpcast.NewBlowfishBlockCrypt(pass)
	case "twofish":
		block, _ = udpcast.NewTwofishBlockCrypt(pass)
	case "cast5":
		block, _ = udpcast.NewCast5BlockCrypt(pass[:16])
	case "3des":
		block, _ = udpcast.NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		block, _ = udpcast.NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		block, _ = udpcast.NewSalsa20BlockCrypt(pass)
	default:
		config.Crypt = "aes"
		block, _ = udpcast.NewAESBlockCrypt(pass)
	}
	if c.Command.Name == "server" {
		log.Println("target:", config.Target)
		listener, err := udpcast.ListenWithOptions(config.Listen, config.Target, config.SockBuf, config.Timeout, block)
		if err != nil {
			log.Fatal(err)
		}

		listener.Start()
	} else {
	}
	return nil
}

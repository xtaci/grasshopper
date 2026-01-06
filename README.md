# 🦗 grasshopper
[![GoDoc][1]][2] [![MIT licensed][3]][4] [![Created At][5]][6] [![Go Report Card][7]][8] [![Release][9]][10]

[1]: https://godoc.org/github.com/xtaci/grasshopper?status.svg
[2]: https://pkg.go.dev/github.com/xtaci/grasshopper
[3]: https://img.shields.io/badge/license-MIT-blue.svg
[4]: LICENSE
[5]: https://img.shields.io/github/created-at/xtaci/grasshopper
[6]: https://img.shields.io/github/created-at/xtaci/grasshopper
[7]: https://goreportcard.com/badge/github.com/xtaci/grasshopper
[8]: https://goreportcard.com/report/github.com/xtaci/grasshopper
[9]: https://img.shields.io/github/v/release/xtaci/grasshopper?color=orange
[10]: https://github.com/xtaci/grasshopper/releases/latest

[English](README.md) | [中文](README_zh.md)

**Grasshopper** is a UDP packet forwarder that listens for incoming packets and forwards them to a configured destination. It optionally supports encryption for both incoming and outgoing packets, using different keys and cryptographic methods.

## Architecture
Grasshopper functions as a chained relay system. For example, consider a chained DNS query:
```
                      ┌────────────┐                 ┌───────────────┐                                 
                      │ ENCRYPTED  │                 │ RE-ENCRYPTION │                                 
                      └──────┬─────┘                 │ AES ───► 3DES │                                 
                             │                       └───┬───────────┘                                 
                             │                           │                                             
                  ┌─────────┐▼             ┌────────────┐│             ┌─────────┐                     
                <HOP0>   HOPS(AES)         │  DECRYPTED │▼          <HOP5>      HOPS(FINAL)            
┌─────────┐       └       ┌────┐           └  DATA   HOPS(3DES)        │       ┌─┴──┐ ┌────────────┐
│ dig xxx ├─► CLEAR TEXT  │HOP1┼── CIPHER ──► PACKET  ┌─┴──┐           └ DNS   │Hop6├─► 8.8.8.8:53 │
│ @hop0   │       ┌       │Hop2│   (AES)   ┌          │Hop4├─ CIPHER ──► QUERY │Hop7│ └────────────┘
└─────────┘       │  ▲    │HOP3│         <HOP2>  ▲    │Hop5│  (3DES)   ┌       └─┬──┘                  
                  │  │    └────┘           │     │    └─┬──┘           │         │                     
                  └──┼──────┘              └─────┼──────┘              └─────────┘                     
                     │                           │                                                     
                  ┌──┼────────┐                  │                                                     
                  │           │                  │                                                     
                  │ OPTIONAL  ├──────────────────┘                                                     
                  │ PACKET    │                                                                        
                  │ PROCESSOR │                                                                        
                  │           │                                                                        
                  └───────────┘                                                                        
```

## Installation

To install the latest version of Grasshopper, use the following command:

```sh
go install  github.com/xtaci/grasshopper/cmd/grasshopper@latest     
```

## Parameters
Grasshopper supports the following command-line parameters:

```text
Grasshopper is a UDP packet forwarder that listens for incoming packets and forwards them to a configured destination. It optionally supports cryptography for both incoming and outgoing packets, using different keys and methods.  Optionally, the listener can be configured to apply cryptogrraphy on both the incoming and outgoing packets, with different keys and methods.

Usage:
  grasshopper [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  start       Start a listener for UDP packet forwarding

Flags:
      --ci string          Cryptography method for incoming data. Available options: aes, aes-128, aes-192, qpp, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "qpp")
      --co string          Cryptography method for outgoing data. Available options: aes, aes-128, aes-192, qpp, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "qpp")
  -c, --config string      config file name
  -h, --help               help for grasshopper
      --ki string          Secret key to encrypt and decrypt for the last hop(client-side) (default "it's a secret")
      --ko string          Secret key to encrypt and decrypt for the next hops (default "it's a secret")
  -l, --listen string      Listener address, eg: "IP:1234" (default ":1234")
  -n, --nexthops strings   Servers to randomly forward to (default [127.0.0.1:3000])
      --sockbuf int        Socket buffer size for the listener (default 1048576)
      --timeout duration   Idle timeout duration for a UDP connection (default 1m0s)
  -t, --toggle             Help message for toggle
  -v, --version            version for grasshopper

Use "grasshopper [command] --help" for more information about a command.
```

## Cryptography Support
- SM4 ([国密](https://en.wikipedia.org/wiki/SM4_(cipher)))
- AES ([Advanced Encryption Standard](https://en.wikipedia.org/wiki/Advanced_Encryption_Standard)), 128, 192, 256-bit
- QPP ([Quantum Permutation Pad](https://epjquantumtechnology.springeropen.com/articles/10.1140/epjqt/s40507-022-00145-y))
- Salsa20 (https://en.wikipedia.org/wiki/Salsa20)
- Blowfish (https://en.wikipedia.org/wiki/Blowfish_(cipher))
- Twofish (https://en.wikipedia.org/wiki/Twofish)
- Cast5 (https://en.wikipedia.org/wiki/CAST-128)
- 3DES (https://en.wikipedia.org/wiki/Triple_DES)
- Tea ([Tiny Encryption Algorithm](https://en.wikipedia.org/wiki/Tiny_Encryption_Algorithm))
- XTea (https://en.wikipedia.org/wiki/XTEA)

## Use Cases

### Case I: Secure Echo

Flow at a glance:

```
Client (ncat @127.0.0.1:4000)
  │ plaintext
  ▼
Level-1 relay (ci=none, co=aes) ── AES cipher ──► Level-2 relay (ci=aes, co=none) ──► UDP echo @127.0.0.1:5000
```

What this means: your terminal only talks to `127.0.0.1:4000`; everything beyond that hop is encrypted on the way out and decrypted before it hits the echo server.

### Step 1: Start a UDP Echo Server

Use `ncat` to start a UDP echo server on port 5000:

```sh
ncat -e /bin/cat -k -u -l 5000
```
### Step 2: Start a Level-2 Relay to the Echo Server

Run the following command to start a relay:

```sh
./grasshopper start --ci aes --co none -l "127.0.0.1:4001" -n "127.0.0.1:5000"
```

- `--ci aes`: Applies encryption to incoming packets.
- `--co none`: Forwards plaintext to the `ncat` echo server.

### Step 3: Start a Level-1 Relay to the Level-2 Relay

Run the following command to start another relay:

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "127.0.0.1:4001"
```

- `--ci none`: No encryption is applied to incoming packets.
- `--co aes`: Encrypts packets and forwards them to the next hop.

### Step 4: Start a Demo Client

Use `ncat` to send UDP packets and interact with the relay chain:

```sh
ncat -u 127.0.0.1 4000
```

### Case II: Secure DNS Query (Random Selection)
Flow at a glance (two relays, random upstream resolver):

```
Laptop dig ──► Level-1 (ci=none, co=aes) ── AES cipher over WAN ──► Level-2 (ci=aes, co=none) ──► DNS pool {8.8.8.8, 1.1.1.1}
```

The laptop only sees `127.0.0.1:4000`; the WAN hop stays encrypted, and Level-2 randomly picks a resolver from the pool for each request.
```
┌──────────── YOUR─LAPTOP ──────────────┐           ┌────────── CLOUD─SERVER ───────────┐
│                                       │           │                                   │
│                                       │           │                                   │
│ ┌───────────────────┐   ┌──────────┐  │           │ ┌──────────┐   ┌───────────────┐  │
│ │                   │   │          │  │           │ │          │   │               │  │
│ │ dig google.com    ├───► Level-1  │  │           │ │ Level-2  ├───► Google DNS:53 │  │
│ │ @127.0.0.1 -p 4000│   │ Relayer  ┼──┼ ENCRYPTED ┼─► Relayer  │   │ CloudFlare:53 │  │
│ │                   │   │          │  │    UDP    │ │          │   │               │  │
│ └───────────────────┘   └──────────┘  │           │ └──────────┘   └───────────────┘  │
│                                       │           │                                   │
│                                       │           │                                   │
└───────────────────────────────────────┘           └───────────────────────────────────┘
```
### Step 1: Start a Level-2 Relay to the DNS Server (On Your Cloud Server 🖥️)

```sh
./grasshopper start --ci aes --co none -l "CLOUD_PUBLIC_IP:4000" -n "8.8.8.8:53,1.1.1.1:53"
```

- `--ci aes`: Decrypts incoming packets from the Level-1 relay. (`ci` stands for cipher-in)
- `--co none`: Forwards decrypted plaintext DNS query packets to the DNS server. (`co` stands for cipher-out)

### Step 2: Start a Level-1 Relay to the Level-2 Relay (On Your Laptop 💻)

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "CLOUD_PUBLIC_IP:4000"
```

- `--ci none`: Since the `dig` command sends queries in plaintext, no decryption is needed for incoming packets.
- `--co aes`: Encrypts and forwards packets to the Level-2 relay.

### Step 3: Query the Level-1 Relay with `dig` (On Your Laptop 💻)

```sh
dig google.com @127.0.0.1 -p 4000
```

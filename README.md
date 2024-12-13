# 🦗 grasshopper
**Grasshopper** is a UDP packet forwarder that listens for incoming packets and forwards them to a configured destination. It optionally supports cryptography for both incoming and outgoing packets, using different keys and methods.

## Architecture
Grasshopper functions as a chained relay system. Take a chained DNS query For example:
```
                      ┌────────────┐                 ┌───────────────┐                                 
                      │ ENCRYPTED  │                 │ RE-ENCRYPTION │                                 
                      └──────┬─────┘                 │ AES ───► 3DES │                                 
                             │                       └───┬───────────┘                                 
                             │                           │                                             
                  ┌─────────┐▼             ┌────────────┐│             ┌─────────┐                     
                <HOP0>   HOPS(AES)         │  DECRYPTED │▼          <HOP5>      HOPS(FINAL)            
┌─────────┐       └       ┌────┐           └  DATA   HOPS(3DES)        │       ┌─┴──┐    ┌────────────┐
│ dig xxx ├─► CLEAR TEXT  │HOP1┼── CIPHER ──► PACKET  ┌─┴──┐           └ DNS   │Hop6├────► 8.8.8.8:53 │
│ @hop0   │       ┌       │Hop2│   (AES)   ┌          │Hop4├─ CIPHER ──► QUERY │Hop7│    └────────────┘
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

Install the latest version of Grasshopper using the following command:

```sh
go install  github.com/xtaci/grasshopper/cmd/grasshopper@latest     
```

## Parameters
Grasshopper supports the following parameters:

```text
Grasshopper is a UDP packet forwarder that listens for incoming packets and forwards them to a configured destination. It optionally supports cryptography for both incoming and outgoing packets, using different keys and methods.  Optionally, the listener can be configured to apply cryptogrraphy on both the incoming and outgoing packets, with different keys and methods.

Usage:
  grasshopper [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  start       Start a listener for UDP packet forwarding

Flags:
      --ci string          Cryptography method for incoming data. Available options: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "3des")
      --co string          Cryptography method for incoming data. Available options: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "3des")
  -h, --help               help for grasshopper
      --ki string          Secret key to encrypt and decrypt for the last hop(client-side) (default "it's a secret")
      --ko string          Secret key to encrypt and decrypt for the next hops (default "it's a secret")
  -l, --listen string      Listener address, eg: "IP:1234" (default ":1234")
  -n, --nexthops strings   Servers to randomly forward to (default [127.0.0.1:3000])
      --sockbuf int        Socket buffer size for the listener (default 1048576)
      --timeout duration   Idle timeout duration for a UDP connection (default 1m0s)
  -t, --toggle             Help message for toggle

Use "grasshopper [command] --help" for more information about a command.
```

## Cases-1 Secure Echo

### Step 1: Start a UDP Echo Server

Use `ncat` to start a UDP echo server on port 5000:

```sh
ncat -e /bin/cat -k -u -l 5000
```
### Step 2: Start a Level-2 Relayer to the Echo Server

Run the following command to start a relayer:

```sh
./grasshopper start --ci aes --co none -l "127.0.0.1:4001" -n "127.0.0.1:5000"
```

- `--ci aes`: Applies cryptography on incoming packets.
- `--co none`: Transfers plaintext to the `ncat` echo server.

### Step 3: Start a Level-1 Relayer to the Level-2 Relayer

Run the following command to start another relayer:

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "127.0.0.1:4001"
```

- `--ci none`: No cryptography is applied to incoming packets.
- `--co aes`: Encrypts and relays packets to the next hop.

### Step 4: Start a Demo Client

Use `ncat` to send UDP packets and interact with the relayer chain:

```sh
ncat -u 127.0.0.1 2132
```

## Cases-2 Secure DNS query
```
┌────────────────── YOUR─LAPTOP ───────────────┐           ┌───────────────── CLOUD─SERVER ────────────┐
│                                              │           │                                           │
│                                              │           │                                           │
│  ┌───────────────────┐                       │           │                                           │
│  │                   │   ┌─────────────────┐ │           │  ┌─────────────────┐   ┌───────────────┐  │
│  │ dig google.com    ┼───►                 │ │           │  │                 │   │               │  │
│  │ @127.0.0.1 -p 4000│   │ Level-1 Relayer ┼─┼ Encrypted ┼──► Level-2 Relayer ┼───► Google DNS:53 │  │
│  │                   │   │                 │ │           │  │                 │   │               │  │
│  └───────────────────┘   └─────────────────┘ │           │  └─────────────────┘   └───────────────┘  │
│                                              │           │                                           │
│                                              │           │                                           │
└──────────────────────────────────────────────┘           └───────────────────────────────────────────┘
```
### Step 1: Start a Level-2 Relayer to the Google DNS Server(On your Cloud Server🖥️)

```sh
./grasshopper start --ci aes --co none -l "CLOUD_PUBLIC_IP:4000" -n "8.8.8.8:53"
```

- `--ci aes`: Decrypts the packet from Level-1 Relayer.
- `--co none`: Transfers decrypted plaintext DNS query packet to Google DNS.

### Step 2: Start a Level-1 Relayer to the Level-2 Relayer(On your Laptop💻)

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "CLOUD_PUBLIC_IP:4000"
```

- `--ci none`: Since `dig` command queries in plaintext, we do not need to decrypt the packet.
- `--co aes`: Decrypts and relays packets to Level-2 Relayer

### Step 3: Query Level-1 Relayer with `dig`(On your Laptop💻)

```sh
dig google.com @127.0.0.1 -p 4000
```

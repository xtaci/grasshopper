# 🦗 grasshopper
The grasshopper will listen for incoming UDP packets and forward them to the configured destination.
Optionally, the listener can be configured to apply cryptogrraphy on both the incoming and outgoing packets, with different keys and methods.

## Architecture
The grasshopper acts like a chained-relayer, for example

```
gh = grasshopper
client --------------> relayer1(gh) --------------> relayer2(gh) -----------------> relayer3(gh) --------------------> destination.
        plaintext                     encrypted                    re-encrypted                        decrypted
```

## Install
```
go install  github.com/xtaci/grasshopper/cmd/grasshopper@latest     
```

## Parameters
```
The grasshopper will listen for incoming UDP packets and forward them to the configured destination.
Optionally, the listener can be configured to apply cryptogrraphy on both the incoming and outgoing packets, with different keys and methods.

Usage:
  grasshopper [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  start       Start a listener for UDP packet forwarding

Flags:
      --ci string          The crytpgraphy method for incoming data, available: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "3des")
      --co string          The crytpgraphy method for outgoing data, available: aes, aes-128, aes-192, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (default "3des")
  -h, --help               help for grasshopper
      --ki string          The secret to encrypt and decrypt for the last hop(incoming) (default "it's a secret")
      --ko string          The secret to encrypt and decrypt for the next hop(outgoing) (default "it's a secret")
  -l, --listen string      listener address, eg: "IP:1234" (default ":1234")
  -n, --nexthops strings   the servers to randomly forward to (default [127.0.0.1:3000])
      --sockbuf int        socket buffer for listener (default 1048576)
      --timeout duration   set how long an UDP connection can live when in idle(in seconds) (default 10m0s)
  -t, --toggle             Help message for toggle

Use "grasshopper [command] --help" for more information about a command.
```

## Example Usage

Step 1: start an UDP echo server with ncat with port 5000
```
ncat -e /bin/cat -k -u -l 5000
```

Step 2: Start the Level-2 relayer to ncat echo 
```
./grasshopper start --ci aes --co none -l "127.0.0.1:4001" -n "127.0.0.1:5000"
--ci aes means we apply cryptography on incoming packets
--co none means we transfer cleartext to ncat echo server
```

Step 3: Start the Level-1 relayer to Level-2 relayer, meanwhile encrypt the packet
```
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "127.0.0.1:4001"
--ci none means we don't apply cryptography on incoming packets
--co aes means we encrypt and relay the encrypted packets to next hop
```

Step 4: Start a demo client, try to type in something.
```
ncat -u 127.0.0.1 2132
```

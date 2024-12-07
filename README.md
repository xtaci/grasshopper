# grasshopper

## Install
```
go install  github.com/xtaci/grasshopper/cmd/grasshopper@latest     
```

## Example Usage

Step 1: start an UDP echo server with ncat with port 5000
```
ncat -e /bin/cat -k -u -l 5000
```

Step 2: Start the Level-2 relayer to ncat echo 
```
./grasshopper start --ci aes --co none -l "127.0.0.1:4001" -n "127.0.0.1:5000
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

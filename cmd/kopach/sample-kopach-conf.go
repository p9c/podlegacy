package main

var sampleKopachConf = `
;rpcuser=      ;;; RPC username
;rpcpass=      ;;; RPC password
;rpcserver=    ;;; RPC server to connect to (default: localhost)
;rpccert=      ;;; RPC server certificate chain for validation (default: /home/loki/.pod/rpc.cert)
;tls=1         ;;; Enable TLS
;proxy=        ;;; Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)
;proxyuser=    ;;; Username for proxy server
;proxypass=    ;;; Password for proxy server
;testnet=1     ;;; Connect to testnet
;simnet=1      ;;; Connect to the simulation test network
;skipverify=1  ;;; Do not verify tls certificates (not recommended!)
;algo=         ;;; Mine with this algorithm, options are blake14lr, cryptonight7v2, keccak, lyra2rev2, scrypt, sha256d, skein, stribog, x11, random or easy
;bench         ;;; Run a benchmark to compare the solution rate for each algorithm on your CPU
;threads=      ;;; Number of threads to spawn (default is -1, meaning all cpu core/threads)
;addr=         ;;; Addresses to put in block coinbases, chosen randomly from all configured addresses         
`

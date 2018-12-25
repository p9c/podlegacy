#!/bin/bash
PORT=11348
for i in $(seq $1 $(podctl -s 127.0.0.1:11348 getblockcount))
do podctl -s 127.0.0.1:11348 getblock `podctl -s 127.0.0.1:11348 getblockhash $i`|grep -e time\" |cut -f2 -d':'|cut -f1 -d","|xargs echo
done

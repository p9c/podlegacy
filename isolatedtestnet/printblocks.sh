#!/bin/bash
for i in $(seq 0 $(podctl -s 127.0.0.1:2008 getblockcount)); do podctl -s 127.0.0.1:2008 getblock `podctl -s 127.0.0.1:2008 getblockhash $i`|grep -e pow_hash\" -e time\" -e bits\" -e height\" -e \"pow_algo\"|cut -f2 -d':'|cut -f1 -d","|xargs echo; done

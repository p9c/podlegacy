go install
podctl -s 127.0.0.1:11348 setgenerate true 1
podctl -s 127.0.0.1:11448 setgenerate true 1
podctl -s 127.0.0.1:11548 setgenerate true 1
podctl -s 127.0.0.1:11648 setgenerate true 1
podctl -s 127.0.0.1:11748 setgenerate true 1
podctl -s 127.0.0.1:11848 setgenerate true 1
podctl -s 127.0.0.1:11948 setgenerate true 1
podctl -s 127.0.0.1:12048 setgenerate true 1
podctl -s 127.0.0.1:12148 setgenerate true 1

go install
podctl -s 127.0.0.1:11348 stop
podctl -s 127.0.0.1:11448 stop
podctl -s 127.0.0.1:11548 stop
podctl -s 127.0.0.1:11648 stop
podctl -s 127.0.0.1:11748 stop
podctl -s 127.0.0.1:11848 stop
podctl -s 127.0.0.1:11948 stop
podctl -s 127.0.0.1:12048 stop
podctl -s 127.0.0.1:12148 stop


go install
podctl -s 127.0.0.1:11348 setgenerate false 1
podctl -s 127.0.0.1:11448 setgenerate false 1
podctl -s 127.0.0.1:11548 setgenerate false 1
podctl -s 127.0.0.1:11648 setgenerate false 1
podctl -s 127.0.0.1:11748 setgenerate false 1
podctl -s 127.0.0.1:11848 setgenerate false 1
podctl -s 127.0.0.1:11948 setgenerate false 1
podctl -s 127.0.0.1:12048 setgenerate false 1
podctl -s 127.0.0.1:12148 setgenerate false 1

for i in $(seq 0 $(podctl -s 127.0.0.1:11348 getblockcount)); do podctl -s 127.0.0.1:11348 getblock `podctl -s 127.0.0.1:11348 getblockhash $i`|grep version\"|cut -f2 -d':'|cut -f1 -d","; done

for i in $(seq 0 $(podctl -s 127.0.0.1:11348 getblockcount)); do podctl -s 127.0.0.1:11348 getblock `podctl -s 127.0.0.1:11348 getblockhash $i`; done

for i in $(seq 0 $(podctl -s 127.0.0.1:11348 getblockcount)); do podctl -s 127.0.0.1:11348 getblock `podctl -s 127.0.0.1:11348 getblockhash $i`|grep pow_hash\"|cut -f2 -d':'|cut -f1 -d","; done

for i in $(seq 0 $(podctl -s 127.0.0.1:11348 getblockcount)); do podctl -s 127.0.0.1:11348 getblock `podctl -s 127.0.0.1:11348 getblockhash $i`|grep time\"|cut -f2 -d':'|cut -f1 -d","; done

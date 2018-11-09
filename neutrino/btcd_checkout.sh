#!/bin/bash

git checkout $(cat $GOPATH/src/github.com/parallelcointeam/pod/neutrino/glide.yaml | grep -A1 btcd | tail -n1 | awk '{ print $2}')

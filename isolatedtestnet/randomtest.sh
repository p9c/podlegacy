#!/bin/bash
clear
#rm -rf node?/testnet
pod --logdir=node0 --algo=random --testnet --configfile=./mining0/config --datadir=./node0 &
pod --logdir=node1 --algo=random --testnet --configfile=./mining1/config --datadir=./node1 &
pod --logdir=node2 --algo=random --testnet --configfile=./mining2/config --datadir=./node2 &

read -n1 -s
killall pod

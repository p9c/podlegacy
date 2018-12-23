#!/bin/bash
clear
rm -rf node0/testnet
pod --algo=random --testnet --configfile=./mining0/config --datadir=./node0 &
rm -rf node1/testnet
pod --algo=random --testnet --configfile=./mining1/config --datadir=./node1 &
rm -rf node2/testnet
pod --algo=random --testnet --configfile=./mining2/config --datadir=./node2 &

read -n1 -s
killall pod

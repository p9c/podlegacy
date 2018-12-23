#!/bin/bash
clear
rm -rf node0/testnet
pod --testnet --configfile=./node0/config --datadir=./node0 &
rm -rf node1/testnet
pod --testnet --configfile=./node1/config --datadir=./node1 &
rm -rf node2/testnet
pod --testnet --configfile=./node2/config --datadir=./node2 &
rm -rf node3/testnet
pod --testnet --configfile=./node3/config --datadir=./node3 &
rm -rf node4/testnet
pod --testnet --configfile=./node4/config --datadir=./node4 &
rm -rf node5/testnet
pod --testnet --configfile=./node5/config --datadir=./node5 &
rm -rf node6/testnet
pod --testnet --configfile=./node6/config --datadir=./node6 &
rm -rf node7/testnet
pod --testnet --configfile=./node7/config --datadir=./node7 &
rm -rf node8/testnet
pod --testnet --configfile=./node8/config --datadir=./node8 &

read -n1 -s
killall pod

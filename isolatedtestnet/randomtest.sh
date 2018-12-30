#!/bin/bash
clear
pod --genthreads=$1 --logdir=mining0 --configfile=./mining0/config --datadir=./mining0 &
pod --genthreads=$1 --logdir=mining1 --configfile=./mining1/config --datadir=./mining1 &
pod --genthreads=$1 --logdir=mining2 --configfile=./mining2/config --datadir=./mining2 &

read -n1 -s
killall pod

#!/bin/bash

PID=$1

WORKING_DIR=$(dirname $0)
cd $WORKING_DIR

sudo rm -rf image
sudo ./c $PID

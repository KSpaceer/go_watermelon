#!/bin/bash

if [ $# -lt 1 ]
then
    echo "No folders are specified!"
    exit 1
fi

if [ ! -f "ca-cert.pem" ]
then
    echo "There is no CA certificate!"
    exit 1
fi

for path in "$@"
do
    cp ca-cert.pem "$path/"
done

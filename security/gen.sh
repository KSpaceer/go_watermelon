#!/bin/bash

if [ $# -lt 1 ]
then
    echo "No folders are specified!"
    exit 1
fi

for path in "$@"
do
    if [ ! -d "$path" ]
    then
        echo "$path" "is not a directory!"
        exit 1
    fi
done

if [ ! -f ca-cert.pem ]
then
   echo "CA certificate doesn't exist!" 
   exit 1
fi

if [ ! -f ca-key.pem ]
then
    echo "CA private key doesn't exist!"
    exit 1
fi

for path in "$@"
do
    # Generate private key and certificate signing request
    openssl req -newkey rsa:4096 -nodes -keyout "$path/key.pem" -out "$path/req.pem" -subj "/CN=WatermelonEnjoyer"

    # Use CA private key to sign the CSR and get back the signed certificate
    openssl x509 -req -in "$path/req.pem" -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out "$path/cert.pem"
done


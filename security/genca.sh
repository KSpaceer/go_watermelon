#!/bin/bash

if [ ! -f ca-cert.pem ]; then
    # Generate CA's private key and self-signed certificate
    openssl req -x509 -newkey rsa:4096 -days 365 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/CN=DailyWatermelon/"

    echo "CA's self-signed certificate"
    openssl x509 -in ca-cert.pem -noout -text
else
    echo "CA's certificate already exists"
fi

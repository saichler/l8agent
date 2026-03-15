#!/usr/bin/env bash

set -e

# Download api.proto dependency (needed for l8api.L8MetaData)
wget -q https://raw.githubusercontent.com/saichler/l8types/refs/heads/main/proto/api.proto -O l8api.proto

# Use the protoc image to run protoc.sh and generate the bindings.
docker run --user "$(id -u):$(id -g)" -e PROTO=l8agent.proto --mount type=bind,source="$PWD",target=/home/proto/ -it saichler/protoc:latest

# Now move the generated bindings to the go/types directory and clean up
rm -rf ../go/types
mkdir -p ../go/types
mv ./types/* ../go/types/.
rm -rf ./types

rm -rf *.rs
rm -f l8api.proto

cd ../go
find . -name "*.go" -type f -exec sed -i 's|"./types/l8agent"|"github.com/saichler/l8agent/go/types/l8agent"|g' {} +

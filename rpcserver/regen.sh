#!/bin/sh

protoc -I. --go_out=plugins=grpc:api api.proto

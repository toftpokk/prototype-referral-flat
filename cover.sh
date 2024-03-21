#!/bin/sh
# Test coverage
go test -coverprofile ./cover/cover.out -cover ./cmd/*
go tool cover -html ./cover/cover.out -o ./cover/cover.html
# output in ./cover/cover.html
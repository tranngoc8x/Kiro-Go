#!/bin/bash
export CGO_ENABLED=0
export TMPDIR=/Users/tranngocthang/webroot/thang/Kiro-Go/tmp
export GOCACHE=/Users/tranngocthang/webroot/thang/Kiro-Go/gocache
/usr/local/go/bin/go test ./...

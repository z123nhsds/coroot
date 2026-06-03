#!/bin/bash
go vet ./... > vet_output.txt 2>&1
cat vet_output.txt

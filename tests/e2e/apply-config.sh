#!/bin/bash

mkdir -p ./tests/temp/data

ls -la
ls -la .local
ls -la .local/data

cp ./tests/e2e/01.config.yaml ./tests/temp/data/config.yaml
#!/usr/bin/env bash

grep 'git' go.mod | grep '\.com' | grep -v indirect | grep -v replace | cut -f 2 | cut -d ' ' -f 1 | while read line; do
  go get -u "$line"
  make vendor
  git add .
  git commit -S -m "update $line"
done

#!/bin/sh

if [ -z ${PKG+x} ]; then echo "PKG is not set"; exit 1; fi
if [ -z ${ROOT_DIR+x} ]; then echo "ROOT_DIR is not set"; exit 1; fi

echo "gofmt:"
OUT=$(gofmt -l $ROOT_DIR)
if [ $(echo -n "$OUT" | wc -l) -ne 0 ]; then echo -n "$OUT"; PROBLEM=1; fi

echo "errcheck:"
OUT=$(errcheck $PKG/...)
if [ $(echo -n "$OUT" | wc -l) -ne 0 ]; then echo -n "$OUT"; PROBLEM=1; fi

echo "go vet:"
OUT=$(go tool vet -all=true -v=true $ROOT_DIR 2>&1 | grep --invert-match -P "(Checking file|\%p of wrong type|can't check non-constant format|not compatible with reflect.StructTag.Get)")
if [ $(echo -n "$OUT" | wc -l) -ne 0 ]; then echo -n "$OUT"; PROBLEM=1; fi

echo "golint:"
OUT=$(golint $PKG/... | grep --invert-match -P "(_string.go:)")
if [ $(echo -n "$OUT" | wc -l) -ne 0 ]; then echo -n "$OUT"; PROBLEM=1; fi

if [ -n "$PROBLEM" ]; then exit 1; fi

SHELL := /bin/bash

build:
	pushd cmd/vault-gcs-lister \
		&& go build \
			-o ../../../build/vault-gcs-lister \
		&& popd

.PHONY: build
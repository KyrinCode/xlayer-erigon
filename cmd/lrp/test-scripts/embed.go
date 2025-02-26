package testscripts

import (
	_ "embed"
)

//go:embed Makefile
var MakefileContent []byte

//go:embed docker-compose.yml
var DockerComposeFileContent []byte

//go:embed xlayerconfig-mainnet.yaml
var XlayerConfigMainnetContent []byte

//go:embed Dockerfile.local
var DockerfileLocalContent []byte

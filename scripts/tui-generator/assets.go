package main

import "embed"

//go:embed templates/*
var generatorAssets embed.FS

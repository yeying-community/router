package app

import "embed"

//go:embed web/build/*
var BuildFS embed.FS

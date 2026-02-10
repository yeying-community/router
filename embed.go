package app

import "embed"

//go:embed web/dist/*
var BuildFS embed.FS

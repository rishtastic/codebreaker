package web

import "embed"

// Files contains the complete browser application.
//
//go:embed templates/* static/*
var Files embed.FS

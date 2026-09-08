// Package corepack embeds the core scenario pack in this directory.
package corepack

import "embed"

// Files is catalog.yaml plus every {section}/*.yaml lab.
//
//go:embed catalog.yaml */*.yaml
var Files embed.FS

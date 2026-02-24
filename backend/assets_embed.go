package beaconassets

import "embed"

// FS contains all UI templates and static files needed by the backend.
//
//go:embed templates/*.html static/*
var FS embed.FS

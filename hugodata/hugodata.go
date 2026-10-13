package hugodata

// Package documentation for hugodata module
//
// This package provides generation of historical JSON data for Hugo static site rendering.
//
// It converts technical JSON data (processed from PFR and boxscores) into Hugo-friendly
// presentation formats. Unlike the technical JSON schemas which are versioned and immutable,
// Hugo presentation data can evolve with template requirements.
//
// Architecture:
//
// Technical JSON (processed/)
//   → pagejson builders (sectioned-page schema) and aggregate builders
//   → Write to output/data/historical/
//
// Hugo processes output/data/ JSON files dynamically during site build, rendering pages
// from data/historical/ instead of pre-generated markdown.
//
// Key principles:
//   - No schema versioning: Hugo presentation data is ephemeral
//   - No invented data: Only transform what exists in processed JSON
//   - No time.Now(): All timestamps are static
//   - Clean separation: Technical models remain untouched

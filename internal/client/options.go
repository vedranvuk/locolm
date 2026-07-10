package client

// This file documents the shared option conventions used across the package.
//
// Every request type embeds the unexported common struct so that the model
// field is available uniformly. Each request type exposes its own typed
// With<Param> helpers (e.g. WithModel, WithTemperature) that operate on that
// concrete request pointer, keeping the functional-option signatures
// type-safe. Cross-cutting client configuration lives on the Client itself
// (WithAPIKey, WithTimeout, WithHTTPClient, WithHeader).

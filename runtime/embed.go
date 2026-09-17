// Package runtime contains the complete, embedded Docker build context.
package runtime

import "embed"

//go:embed Dockerfile golf-shell golf-brief golf-check golf-hint
var Files embed.FS

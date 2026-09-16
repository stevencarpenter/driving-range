// Package runtime contains the complete, embedded Docker build context.
package runtime

import "embed"

//go:embed Dockerfile golf-shell golf-brief
var Files embed.FS

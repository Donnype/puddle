package dockerfiles

import "embed"

//go:embed go python java node rust php ruby
var FS embed.FS

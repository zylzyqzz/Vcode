// Command vcode is a config- and plugin-driven coding agent CLI.
package main

import (
	"os"

	"vcode/internal/cli"

	// Blank imports wire compile-time built-ins into their registries.
	_ "vcode/internal/provider/anthropic"
	_ "vcode/internal/provider/openai"
	_ "vcode/internal/tool/builtin"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}

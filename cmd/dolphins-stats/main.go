// Command dolphins-stats generates statistics content from processed JSON. It
// has no download or parse commands: programs that need them build on package
// cli and register their own.
package main

import "github.com/Mundo-Dolphins/mundodolphins-stats/cli"

func main() { cli.Execute() }

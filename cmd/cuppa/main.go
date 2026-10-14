// Command cuppa posts a daily watercooler question to team chat.
//
// This is a placeholder so the build and CI have something to check.
// The real CLI arrives in milestone M3.
package main

import "fmt"

// version is set at build time by the release tooling.
// Local builds show "dev".
var version = "dev"

func main() {
	fmt.Println("cuppa " + version)
}

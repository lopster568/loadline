// Command loadline-interposer relays a stdio MCP session and logs every
// JSON-RPC frame to JSONL.
//
// Usage:
//
//	loadline-interposer --log <path> [--full-results] -- <server command...>
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lopster568/loadline/interposer"
)

func main() {
	logPath := flag.String("log", "", "path to the JSONL frame log (required, append-only)")
	fullResults := flag.Bool("full-results", false, "log complete response payloads instead of summaries")
	showVersion := flag.Bool("version", false, "print the interposer version and exit")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"loadline-interposer %s\n\nUsage:\n  loadline-interposer --log <path> [--full-results] -- <server command...>\n\nFlags:\n",
			interposer.Version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(interposer.Version)
		return
	}

	serverCmd := flag.Args()
	if *logPath == "" || len(serverCmd) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	code, err := interposer.Run(interposer.Options{
		LogPath:     *logPath,
		FullResults: *fullResults,
		ServerCmd:   serverCmd,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "loadline-interposer:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

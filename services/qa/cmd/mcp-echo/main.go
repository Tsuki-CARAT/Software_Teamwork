package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient/testserver"
)

func main() {
	addr := flag.String("addr", ":8099", "listen address")
	flag.Parse()

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return testserver.EchoServer()
	}, nil)

	log.Printf("MCP echo server listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}

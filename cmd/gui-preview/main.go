// Command gui-preview serves every cmd/gui screen fixture over HTTP for
// manual review in any browser, and is what tools/visual's Playwright
// script drives to capture the screenshot-regression baselines — neither
// job needs Windows or walk, which is why this lives outside cmd/gui's own
// windows-tagged files.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/labsio-app/aion-theory-sync/internal/webui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4173", "address to listen on")
	flag.Parse()

	handler := webui.NewPreviewHandler(webui.Fixtures())
	fmt.Printf("Aperçu des écrans sur http://%s/\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// Minimal service used by scripts/e2e.sh to exercise a generated project.
package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from kuberLaunch e2e (REDIS_URL=%s)\n", os.Getenv("REDIS_URL"))
	})
	_ = http.ListenAndServe(":8080", nil)
}

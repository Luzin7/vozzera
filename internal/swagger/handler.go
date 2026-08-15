package swagger

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFS embed.FS

//go:embed openapi.yaml
var openapiYAML []byte

func RegisterHandlers(mux *http.ServeMux) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiYAML)
	})
}

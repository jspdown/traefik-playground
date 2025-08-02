package traefik

import (
	"net/http"
	"net/http/httptest"
)

// Whoami is a fake server responding 418 Teapot with the raw request.
type Whoami struct{}

// NewWhoami creates a new Whoami.
func NewWhoami() *httptest.Server {
	s := &Whoami{}

	handler := http.NewServeMux()
	handler.HandleFunc("/", s.handle)

	return httptest.NewServer(handler)
}

func (s *Whoami) handle(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusTeapot)

	if err := req.Write(rw); err != nil {
		http.Error(rw, "", http.StatusInternalServerError)

		return
	}
}

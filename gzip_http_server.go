package justproxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

type gzipHttpServer struct {
	port       int
	enableGzip bool

	serveHTTP func(w http.ResponseWriter, r *http.Request)
}

var _server *http.Server // preventing unwilled garbage collection

func (srv *gzipHttpServer) start() error {
	_server = &http.Server{
		Addr:    fmt.Sprintf(":%d", srv.port),
		Handler: http.HandlerFunc(getHandler(srv)),
	}

	return _server.ListenAndServe()
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// gzip.Reader.Close() does not close underlying reader, so we need to close at the end.
type gzipReader struct {
	*gzip.Reader
}

type gzipBodyReader struct {
	io.ReadCloser
	gzipReader
}

func (r gzipBodyReader) Read(p []byte) (int, error) {
	return r.gzipReader.Read(p)
}

func (r gzipBodyReader) Close() error {
	r.gzipReader.Close()
	return r.ReadCloser.Close()
}

func getHandler(srv *gzipHttpServer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// handling accept-decooding: gzip
		if !srv.enableGzip || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || !isGzipAble(r) {
			srv.serveHTTP(w, r)
		} else {
			w.Header().Add("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			srv.serveHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
		}
	}
}

func isGzipAble(r *http.Request) bool {
	ext := filepath.Ext(r.RequestURI)
	ext = strings.ToUpper(ext)

	switch ext {
	case ".PNG", ".JPG", ".JPEG", ".IPA", ".PLIST":
		return false
	}

	return true
}

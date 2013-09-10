package justproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
)

type ProxyServer struct {
	gzipHttpServer
	listenPort   int
	gzip         bool
	keepalive    bool
	localCaching bool

	serveFunc func(w http.ResponseWriter, r *http.Request)
}

func (proxy *ProxyServer) Start() error {
	err := proxy.start()
	if err != nil {
		return err
	}

	return nil
}

func (proxy *ProxyServer) Proxying(w io.Writer, r *http.Request, redirectAddr string) (int64, error) {
	c, err := net.Dial("tcp", redirectAddr)
	if err != nil {
		return 0, fmt.Errorf("while dial: %v", err)
	}

	defer c.Close()

	httpCli := httputil.NewClientConn(c, nil)

	// creating new request
	newR, err := http.NewRequest(r.Method, r.RequestURI, r.Body)
	if err != nil {
		return 0, fmt.Errorf("while creating HTTP request to target: %v", err)
	}

	newR.Host = r.Host

	// copying HTTP header & cookie
	for k, v := range r.Header {
		// fmt.Println(k, v)

		if !proxy.localCaching {
			switch k {
			case "If-Modified-Since", "If-None-Match":
				continue
			}
		}

		for i, s := range v {
			if i == 0 {
				newR.Header.Set(k, s)
			} else {
				newR.Header.Add(k, s)
			}
		}
	}

	cookies := r.Cookies()
	for _, cookie := range cookies {
		newR.AddCookie(cookie)
	}

	// resp, err := httpCli.Do(newR)
	resp, _ := httpCli.Do(newR)

	defer safelyDo(func() { resp.Body.Close() })

	w2, ok := w.(http.ResponseWriter)
	if ok {
		safelyDo(func() {
			for k, v := range resp.Header {
				// fmt.Println(k, v)
				for i, s := range v {
					if i == 0 {
						w2.Header().Set(k, s)
					} else {
						w2.Header().Add(k, s)
					}
				}
			}
		})

		safelyDo(func() { w2.WriteHeader(resp.StatusCode) })
	}

	written, err := io.Copy(w, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("while write: %v", err)
	}

	return written, nil
}

func NewProxyServer(listenPort int, gzip, keepalive, localCaching bool, serveFunc func(w http.ResponseWriter, r *http.Request)) *ProxyServer {
	proxy := &ProxyServer{
		listenPort:   listenPort,
		gzip:         gzip,
		keepalive:    keepalive,
		localCaching: localCaching,
		serveFunc:    serveFunc,
	}

	serveHTTP := func(w http.ResponseWriter, r *http.Request) {
		proxy.serveFunc(w, r)
	}

	httpServer := gzipHttpServer{
		port:       listenPort,
		enableGzip: gzip,
		serveHTTP:  serveHTTP,
	}

	proxy.gzipHttpServer = httpServer

	return proxy
}

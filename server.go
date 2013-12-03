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

	ServeFunc func(w http.ResponseWriter, r *http.Request)
}

func (proxy *ProxyServer) Start() error {
	err := proxy.start()
	if err != nil {
		return err
	}

	return nil
}

func (proxy *ProxyServer) Proxying(w io.Writer, r *http.Request, redirectAddr string) (int64, error) {
	reqPipeFunc := func(r io.Reader) (io.Reader, error) {
		return r, nil
	}

	respPipeFunc := func(_w io.Writer, _r io.Reader) (int64, error) {
		return io.Copy(_w, _r)
	}

	return proxy.ProxyingWithCopyFunc(w, r, redirectAddr, reqPipeFunc, respPipeFunc)
}

func (proxy *ProxyServer) ProxyingWithCopyFunc(w io.Writer, r *http.Request, redirectAddr string,
	reqPipeFunc func(r io.Reader) (io.Reader, error),
	respPipeFunc func(w io.Writer, r io.Reader) (int64, error)) (int64, error) {

	c, err := net.Dial("tcp", redirectAddr)
	if err != nil {
		return 0, fmt.Errorf("while dial: %v", err)
	}

	defer c.Close()

	httpCli := httputil.NewClientConn(c, nil)

	reqBody, err := reqPipeFunc(r.Body)
	if err != nil {
		return 0, fmt.Errorf("while pipe req: %v", err)
	}

	// creating new request
	newR, err := http.NewRequest(r.Method, r.RequestURI, reqBody)
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
	resp, err := httpCli.Do(newR)
	if resp == nil {
		if err != nil {
			panic(err.Error())
		} else {
			panic("unknown error: thus response object is nil")
		}
	}

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

	written, err := respPipeFunc(w, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("while pipe resp: %v", err)
	}

	return written, nil
}

func NewProxyServer(listenPort int, gzip, keepalive, localCaching bool) *ProxyServer {
	proxy := &ProxyServer{
		listenPort:   listenPort,
		gzip:         gzip,
		keepalive:    keepalive,
		localCaching: localCaching,
	}
	proxy.ServeFunc = func(w http.ResponseWriter, r *http.Request) {
		addr := GetAddr(r.Host)
		_, err := proxy.Proxying(w, r, addr)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	serveHTTP := func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeFunc(w, r)
	}

	httpServer := gzipHttpServer{
		port:       listenPort,
		enableGzip: gzip,
		serveHTTP:  serveHTTP,
	}

	proxy.gzipHttpServer = httpServer

	return proxy
}

package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"bytes"
	"fmt"
	"os"
	"strings"
	"net/url"
	"strconv"
)

const (
	URL_PREFIX        = "/"
	SPARK_MASTER_HOST = "127.0.0.1:8080"
)

var sparkMasterAddr string

func main() {

	fmt.Println(os.Args[1])
	sparkMasterAddr = os.Args[1]
	if len(sparkMasterAddr) == 0 {
		sparkMasterAddr = SPARK_MASTER_HOST
	}

	http.HandleFunc("/healthz", Healthz)
	http.ListenAndServe(":9999", http.HandlerFunc(proxyHandleFunc))
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func proxyHandleFunc(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("Received request %s %s \n", req.Method, req.URL.String())
	if req.URL.Path == "" || req.URL.Path == URL_PREFIX {
		http.Redirect(w, req, URL_PREFIX+"proxy:"+ sparkMasterAddr, 302)
		return
	}

	targetHost, targetPath := extractURLDetails(req.URL.Path)

	// replace out-request with target info
	outReq := new(http.Request)
	*outReq = *req
	outReq.URL.Path = targetPath
	outReq.URL.Host = targetHost
	outReq.URL.Scheme = "http"
	outReq.Header.Set("Accept-Encoding", "")

	res, err := http.DefaultTransport.RoundTrip(outReq)
	if err != nil {
		fmt.Printf("transport outReq error: %s \n", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer res.Body.Close()

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	if res.StatusCode == 200 {
		w.WriteHeader(res.StatusCode)
		if IsStaticPath(targetPath) {
			io.Copy(w, res.Body)
		} else {
			data := rewriteLinks(res.Body, targetHost)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data)
		}
		return
	} else if res.StatusCode == 302 {
		locationURL, err := url.Parse(res.Header.Get("Location"))
		if err != nil {
			http.Redirect(w, req, URL_PREFIX+"proxy:"+ sparkMasterAddr, 302)
		}
		rURL := URL_PREFIX + "proxy:" + outReq.URL.Host + locationURL.Path + "?" + locationURL.Query().Encode()
		http.Redirect(w, req, rURL, 302)
		return
	}
	return
}

func rewriteLinks(src io.Reader, targetHost string) []byte {
	data, err := ioutil.ReadAll(src)
	if err != nil {
		fmt.Printf("proxy ioutil read failed, error:%s \n", err.Error())
		return nil
	}
	target := fmt.Sprintf("%sproxy:%s/", URL_PREFIX, targetHost)
	page := bytes.Replace(data, []byte(`href="/`), []byte(`href="`+target), -1)
	page = bytes.Replace(page, []byte(`href="log`), []byte(`href="`+target+"log"), -1)
	page = bytes.Replace(page, []byte(`href="http://`), []byte(`href="`+URL_PREFIX+"proxy:"), -1)
	page = bytes.Replace(page, []byte(`src="/`), []byte(`src="`+target), -1)
	page = bytes.Replace(page, []byte(`action="`), []byte(`action="`+target), -1)
	return page
}
func extractURLDetails(path string) (string, string) {
	start, idx := 0, 0
	targetHost, proxyPath := "", ""
	if strings.HasPrefix(path, URL_PREFIX+"proxy:") {
		start = len(URL_PREFIX) + 6 // len('proxy:') == 6

		idx = strings.Index(path[start:], "/")
		if idx == -1 {
			targetHost = path[start:]
			return targetHost, proxyPath
		}
		targetHost = path[start:(idx + start)]
		proxyPath = path[(idx + start):]
		return targetHost, proxyPath
	}

	return sparkMasterAddr, path
}

func IsStaticPath(path string) bool {
	staticFileSuffixs := []string{"js", "png", "css"}
	for _, v := range staticFileSuffixs {
		if strings.HasSuffix(path, v) {
			return true
		}
	}
	return false
}

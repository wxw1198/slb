//对应doWorker模式
package strategy

import (
	"common/utils"

	"net/http"
	"net/http/httputil"
)

// should be modified
func DoWorkForCustomer(w http.ResponseWriter, req *http.Request, strIP string) {

	//跨域

	if origin := req.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	utils.Log.Debug("DoWorkForCustomer 111 original received then send to req.URL.Host:%s", req.URL.Host)
	director := func(req *http.Request) {
		req = req
		req.URL.Scheme = "http"
		req.URL.Host = strIP
	}
	utils.Log.Debug("received then send to server:%s", strIP)
	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(w, req)

}

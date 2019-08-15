//监听来自客户端的选机请求
package engine

import (
	"common/utils"

	"fmt"
	"net/http"

	"slb/config"

	"slb/strategy"

	"time"
	//"github.com/gorilla/mux"
	//"github.com/urfave/negroni"
)

//接口

var route string = "/"

var selectServer string = "/yfy/select/lb/server"
var severState string = "/yfy/server/state"
var userPolicy string = "/yfy/user/policy"
var configFile string = "/yfy/server/configinfo"

type slb struct {
	si strategy.StrategyInterface //调度策略接口
	cf *config.Configuration      //全局配置文件
}

func NewSlb() *slb {
	return &slb{si: strategy.NewStrategy(), cf: nil}
}

func (s *slb) Run() {

	go s.si.Run()

	/*m := mux.NewRouter()
	//m.Handle(route, recoverWrap(http.HandlerFunc(s.dealRoute)))
	m.Handle(selectServer, recoverWrap(http.HandlerFunc(s.dealReqServer)))
	m.Handle(severState, recoverWrap(http.HandlerFunc(s.dealServerState)))
	m.Handle(userPolicy, recoverWrap(http.HandlerFunc(s.dealUserPolicy)))
	m.Handle(configFile, recoverWrap(http.HandlerFunc(s.dealUpdateConfig)))
	n := negroni.Classic()
	n.UseHandler(m)
	port := utils.ConfigFile.Read_string("local_server", "listen_port", "8080")
	//不需要指定本地IP
	listenStr := fmt.Sprintf("%s:%s", "0.0.0.0", port)
	fmt.Println(listenStr)
	// 2 监听服务开启
	n.Run(listenStr)*/

	// 支持接收所有信息
	http.HandleFunc(route, s.dealRoute) //分享调度接口
	port := utils.ConfigFile.Read_string("local_server", "listen_port", "8080")
	listenStr := fmt.Sprintf("%s:%s", "0.0.0.0", port)
	err := http.ListenAndServe(listenStr, nil)
	if err != nil {
		fmt.Println("ListenAndServe error: %s", err.Error())
	}

}

func recoverWrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//var err error
		defer utils.DealPanic()

		fmt.Println("recover wrap url:", r.URL.Path)

		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers",
				"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}

		h.ServeHTTP(w, r)
	})
}

func (s *slb) dealRoute(w http.ResponseWriter, r *http.Request) {

	defer utils.DealPanic()

	if selectServer == r.URL.Path {

		utils.Log.Debug("Match selectServer success,%s", r.URL.Path)
		s.dealReqServer(w, r)

	} else if severState == r.URL.Path {

		utils.Log.Debug("Match dealServerState  success,%s", r.URL.Path)
		s.dealServerState(w, r)

	} else if userPolicy == r.URL.Path {

		utils.Log.Debug("Match dealUserPolicy success,%s", r.URL.Path)
		s.dealUserPolicy(w, r)

	} else if configFile == r.URL.Path {
		utils.Log.Debug("Match dealUpdateConfig success,%s", r.URL.Path)
		s.dealUpdateConfig(w, r)
	} else {
		utils.Log.Debug("Match other success,%s", r.URL.Path)
		s.dealReqServer(w, r)
	}
}

func (s *slb) dealReqServer(w http.ResponseWriter, r *http.Request) {

	defer utils.DealPanic()

	body := strategy.NewReqSlbTask()

	utils.Log.Debug("come in to dealReqServer")

	//没有body也默认使用cpu模式和代理模式

	//if b := utils.ParseReqBodyToJsonUnclosed(r, body, true); !b {
	//	w.WriteHeader(http.StatusBadRequest)
	//	return
	//}

	utils.ParseReqBodyToJsonUnclosed(r, body, true)
	if "" == body.TaskType {
		body.TaskType = "cpu"
	}
	if "" == body.ReqMode {
		body.ReqMode = strategy.SelectServer
	}

	if body.ReqMode == strategy.SelectServer || body.ReqMode == strategy.DoWork {
		s.si.AddSlbReq(body)
		select {
		//给客户的回复
		case res := <-*body.ResponseChan:
			//302 answer
			close(*body.ResponseChan)
			if body.ReqMode == strategy.SelectServer {
				//w.Write([]byte(res))

				if "" == res {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					url := "http://" + res + r.RequestURI
					fmt.Println(url) //这里还是：http://yourdomain.com
					http.Redirect(w, r, url, http.StatusFound)
				}

				//do work for customer
			} else if body.ReqMode == strategy.DoWork {
				strategy.DoWorkForCustomer(w, r, res)
			}
		//回复超时
		case <-time.After(time.Second * 10):
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		fmt.Println("not support this mode")
		utils.Log.Debug("not support this mode")
	}
}

func (s *slb) dealServerState(w http.ResponseWriter, r *http.Request) {

	defer utils.DealPanic()

	body := &strategy.ServerState{}
	if b := utils.ParseReqBodyToJson(r, body, true); !b {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println("parse body err")
		return
	}

	s.si.UpdateServerState(body)
}

func (s *slb) dealUserPolicy(w http.ResponseWriter, r *http.Request) {

	defer utils.DealPanic()

	body := strategy.NewUserPolicy()
	if b := utils.ParseReqBodyToJson(r, body, true); !b {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.si.UpdateUserPolicy(body)

	select {
	//给客户的回复
	case res := <-*body.ResponseChan:
		close(*body.ResponseChan)
		w.Write([]byte(res))
	//回复超时
	case <-time.After(time.Second * 5):
		w.WriteHeader(http.StatusInternalServerError)

		//http.Redirect()
	}
}

func (s *slb) dealUpdateConfig(w http.ResponseWriter, r *http.Request) {

	defer utils.DealPanic()

	utils.Log.Debug("received req: UpdateConfig ")

	body := &config.Configuration{}
	if b := utils.ParseReqBodyToJson(r, body, true); !b {
		w.WriteHeader(http.StatusBadRequest)
		utils.Log.Debug("ParseReqBodyToJson error ")
		return
	}
	s.si.UpdateConfig(body)
}

package healthcheck

import (
	"fmt"
	"net"
	//"os"
	//"io"
	"bytes"
	"common/config/goini"
	"common/utils"
	"encoding/json"

	"net/http"
	"slb/config"
	"slb/healthcheck_server"
	//"slb/strategy"
	"io/ioutil"
	"strconv"
	"sync"
	"time"
)

//发送给slb的结构体类型
type SendToSlbInfo struct {
	Ip       string
	CupUtil  int
	IoWait   int
	MemUtil  int
	MemTotal int64
	Down     bool
}

// 构造对象的调用,使用匿名组合
type CheckTask struct {
	generalHealthInfo
	listenPort string //http的监听端口
	lock       sync.Mutex
}

type generalHealthInfo []*singleHealthInfo

//服务器的链路状态
type linkState struct {
	failed            bool //获取服务器健康状态，server不在线标记，
	active            bool //给server 发送健康状态请求，连续响应三次以上，标记为true， 连续没有响应三次以上，标记为false ，该状态要发送给engine
	inactiveTries     int  //给server 发送健康状态请求，server连续3次没有响应，记active为false
	activeTries       int  //给server 发送健康状态请求，server连续3次有响应，记active为true
	retryNum          int  //给server 发送健康状态请求，server响应或不响应的次数。一般为3次
	heartbeatInterval int  //给server 发送健康状态请求的时间间隔
}

//服务器的健康状态
type healthState struct {
	cupUtil  int
	ioWait   int
	memUtil  int
	memTotal int64
}
type serverAddr struct {
	serverIp   string
	serverPort int
}

//需要获取健康状态的地址列表
type singleHealthInfo struct {
	slbUrl     string
	serveraddr serverAddr
	healthinfo healthState
	linkinfo   linkState
}

func (sender *CheckTask) Run() {
	sender.SetUp()
	sender.StartCheckingHealth()
	utils.Log.Error("CheckTask %v", sender)
}

func byteString(p []byte) string {

	defer utils.DealPanic()
	for i := 0; i < len(p); i++ {
		if p[i] == 0 {
			return string(p[0:i])
		}
	}
	return string(p)
}

func getBody(r *http.Request, bodyStruct interface{}, delBody bool) bool {
	defer utils.DealPanic()
	if delBody {
		defer r.Body.Close()
	}
	if r.ContentLength <= 0 {
		fmt.Println("read body error1:")
		return false
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("read body error2:")
		return false
	}
	str := byteString(body)
	utils.Log.Info("in getBody body:%s", str)
	err = json.Unmarshal([]byte(str), &bodyStruct)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println("read body error3:")
		return false
	}
	fmt.Printf("%+v", bodyStruct)
	return true
}

func (s *CheckTask) dealReceiveConfig(w http.ResponseWriter, req *http.Request) {
	defer utils.DealPanic()

	utils.Log.Info("dealReceiveConfig...")
	fmt.Println("dealReceiveConfig...")
	//跨域
	if origin := req.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}
	utils.Log.Info("com in synchronous  received\n ")

	var tmp_json config.Configuration

	if b := getBody(req, &tmp_json, true); !b {

		utils.Log.Info("set_seatstate get body err")
	} else {
		// 往内存里面存数据
		utils.Log.Info("dealReceiveConfig  tmp_json= %v", tmp_json)
		fmt.Println("dealReceiveConfig  tmp_json= %v", tmp_json)

		// 更新内存数据
		s.lock.Lock()
		s.generalHealthInfo = s.generalHealthInfo[0:0]
		for i := 0; i < len(tmp_json.Backends); i++ {
			tempSingleHealthInfo := newSingleHealthInfo()
			s.generalHealthInfo = append(s.generalHealthInfo, tempSingleHealthInfo)
			s.generalHealthInfo[i].serveraddr.serverIp = tmp_json.Backends[i].Host
			s.generalHealthInfo[i].serveraddr.serverPort = tmp_json.Backends[i].Healthcheckport

			s.generalHealthInfo[i].linkinfo.failed = true
			s.generalHealthInfo[i].linkinfo.active = false
			s.generalHealthInfo[i].linkinfo.retryNum = tmp_json.Backends[i].Healthcheck.RetryNum
			s.generalHealthInfo[i].linkinfo.heartbeatInterval = tmp_json.Backends[i].Healthcheck.HeartbeatInterval
			s.generalHealthInfo[i].slbUrl = tmp_json.Glob.SlbStateInterface
			utils.Log.Info("len(tmp_json.Backends)=%d, serverIp =%s, serverPort = %d,retryNum = %d,heartbeatInterval = %d,slbUrl = %s ",
				len(tmp_json.Backends),
				s.generalHealthInfo[i].serveraddr.serverIp, s.generalHealthInfo[i].serveraddr.serverPort,
				s.generalHealthInfo[i].linkinfo.retryNum, s.generalHealthInfo[i].linkinfo.heartbeatInterval,
				s.generalHealthInfo[i].slbUrl)
		}
		s.lock.Unlock()

	}

}

//开启死循环定期查询各个服务器
func (s *CheckTask) StartCheckingHealth() {

	defer utils.DealPanic()
	// 重新启动go协程开启查询健康状态的udp请求
	// 开启循环发送数据
	go s.CheckHealth()
	// 开启监听
	http.HandleFunc("/yfy/server/configinfo", s.dealReceiveConfig) //查询一段时间内的任务状态
	utils.Log.Info("Regedit User Get Push Url Function")

	// 服务器要监听的主机地址和端口号
	listenString := "0.0.0.0:" + s.listenPort
	err := http.ListenAndServe(listenString, nil)
	if err != nil {
		utils.Log.Info("ListenAndServe error: %s", err.Error())
	}
}

func (s *CheckTask) CheckHealth() {
	defer utils.DealPanic()

	for {
		s.lock.Lock()
		utils.Log.Info("CheckHealth  len(s.generalHealthInfo)= %d ", len(s.generalHealthInfo))
		for i := 0; i < len(s.generalHealthInfo); i++ {

			utils.Log.Info("cycle send serverIp  = %s", s.generalHealthInfo[i].serveraddr.serverIp)
			s.generalHealthInfo[i].SendRequest()
		}
		utils.Log.Info("CheckHealth  len(s.generalHealthInfo)= %d", len(s.generalHealthInfo))
		s.lock.Unlock()
		time.Sleep(time.Second * 5)
	}
}

func (s *CheckTask) SetUp() {

	defer utils.DealPanic()

	config_ptr := goini.Init("config.ini")
	s.listenPort = config_ptr.Read_string("WEB", "listen_port", "")
}

func NewCheckTask() *CheckTask {
	return &CheckTask{}
}

func newHealthState() healthState {
	return healthState{
		cupUtil:  0,
		ioWait:   0,
		memUtil:  0,
		memTotal: 0,
	}
}

func newLinkState() linkState {
	return linkState{
		failed:        false,
		active:        true,
		inactiveTries: 0,
		activeTries:   0,
	}
}

func newServerAddr() serverAddr {
	return serverAddr{
		serverIp:   "",
		serverPort: 80,
	}
}

func newSingleHealthInfo() *singleHealthInfo {
	return &singleHealthInfo{
		healthinfo: newHealthState(),
		linkinfo:   newLinkState(),
		serveraddr: newServerAddr(),
	}
}

//给engine发送应答
func (s *singleHealthInfo) SendRespenseToEngine() {

	defer utils.DealPanic()
	// 构造发送body

	var sendBody SendToSlbInfo
	sendBody.CupUtil = s.healthinfo.cupUtil
	sendBody.IoWait = s.healthinfo.ioWait
	sendBody.MemUtil = s.healthinfo.memUtil
	sendBody.MemTotal = s.healthinfo.memTotal
	sendBody.Ip = s.serveraddr.serverIp
	sendBody.Down = !s.linkinfo.active
	jsoned_string, _ := json.Marshal(sendBody)
	body := bytes.NewBuffer([]byte(jsoned_string))

	utils.Log.Debug("postUrl= %s", s.slbUrl)
	utils.Log.Debug("postbody= %s", body)

	req, err := http.NewRequest("POST", s.slbUrl, body)
	//req.Header.Set("X-Custom-Header", "myvalue")
	//req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		utils.Log.Error("SendRespenseToEngine error = %v", err)
	}
	resp.Body.Close()
	utils.Log.Info("SendRespenseToEngine success = %v", err)

}

//发送查询健康状态请求给服务器
func (s *singleHealthInfo) SendRequest() {
	defer utils.DealPanic()

	//defer utils.DealPanic()
	//给服务器发送udp请求
	addr, err := net.ResolveUDPAddr("udp", s.serveraddr.serverIp+":"+strconv.Itoa(s.serveraddr.serverPort))
	if err != nil {
		utils.Log.Error("net.ResolveUDPAddr fail.", err)
		return
	}
	socket, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		utils.Log.Error("net.DialUDP fail.", err)
		return
	}
	t := time.Now()
	socket.SetDeadline(t.Add(time.Duration(5 * time.Second)))
	defer socket.Close()
	_, err = socket.Write([]byte("checkhealth"))

	// 接收服务器的udp回复
	data := make([]byte, 512)
	lens, remoteAddr, err := socket.ReadFromUDP(data)
	utils.Log.Info("remoteAddr: %v", remoteAddr)

	// 判断是否发送活跃信息及健康信息给slb
	var bSend bool

	if err != nil {
		utils.Log.Error("error recv data,err:%v", err)
		bSend = s.inActiveMark()
	} else {
		data = data[:lens]
		bSend = s.activeMark(data)
	}

	//发送信息给slb
	utils.Log.Info("bSend:%v", bSend)

	if bSend {
		s.SendRespenseToEngine()
	}

}

func (s *singleHealthInfo) activeMark(data []byte) bool {

	defer utils.DealPanic()
	var send bool
	if s.linkinfo.activeTries >= s.linkinfo.retryNum {

		if s.linkinfo.failed {
			s.linkinfo.failed = false
			utils.Log.Error("Server [%s] come to active", s.serveraddr.serverIp)
		}
		if !s.linkinfo.active {
			//不活跃状态，突然活跃了要发给engine
			send = true
		} else {
			//一直是活跃状态就不用发了，但是真的不用发了吗。如果健康状态改变了还是要发，增加判断。
			//把收到的json数据转化为结构体填入各个服务器对应的字段中
			//分解body内容
			ss := &healthcheck_server.ServerState{}
			str := string(data)
			fmt.Printf("str =%s\n ", str)
			err := json.Unmarshal([]byte(str), ss)
			if err != nil {
				utils.Log.Error("Unmarshal failed\n", err)
				send = false
			} else {
				utils.Log.Error("Unmarshal result\n", ss.CupUtil, ss.IoWait, ss.MemUtil, ss.MemTotal)
				if s.healthinfo.cupUtil != ss.CupUtil || s.healthinfo.ioWait != ss.IoWait || s.healthinfo.memUtil != ss.MemUtil {
					send = true
				} else {
					send = false
				}
				s.healthinfo.cupUtil = ss.CupUtil
				s.healthinfo.ioWait = ss.IoWait
				s.healthinfo.memTotal = ss.MemTotal
				s.healthinfo.memUtil = ss.MemUtil
			}
		}
		s.linkinfo.active = true
		s.linkinfo.inactiveTries = 0
	} else {

		utils.Log.Error(" address[%s] tries [%d] is  online ", s.serveraddr.serverIp, s.linkinfo.activeTries)
		s.linkinfo.activeTries++
		send = false
	}
	return send
}

func (s *singleHealthInfo) inActiveMark() bool {
	defer utils.DealPanic()
	var send bool
	if s.linkinfo.inactiveTries >= s.linkinfo.retryNum {
		utils.Log.Error("server  inactive [%s]", s.serveraddr.serverIp)
		if s.linkinfo.active {
			//上一次是活跃状态转为不活跃状态，要发送
			send = true
		} else {
			//本来就是不活跃状态，就不用发送了
			send = false
		}
		s.linkinfo.active = false
		s.linkinfo.activeTries = 0
	} else {

		//启动时就是不活跃状态，那也得发
		send = true
		s.linkinfo.failed = true
		s.linkinfo.inactiveTries++
		utils.Log.Error(" address[%s] tries [%d] is not online ", s.serveraddr.serverIp, s.linkinfo.inactiveTries)
	}
	utils.Log.Error(" inActiveMark server %s,s.linkinfo.active = %v, send = %v", s.serveraddr.serverIp, s.linkinfo.active, send)
	return send
}

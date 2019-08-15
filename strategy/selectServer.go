//选择合适的服务器

package strategy

import (
	"common/utils"
	"encoding/json"

	"fmt"
	"regexp"

	"slb/config"
	"strconv"
	"strings"
	"sync"
)

type StrategyInterface interface {

	//启动服务器
	Run()

	//服务器退出
	Quit()

	//添加新的请求
	AddSlbReq(r *ReqSlbTask)

	//更新用户策略
	UpdateUserPolicy(up *UserPolicy)

	//更新config
	UpdateConfig(up *config.Configuration)

	//更新服务的状态
	UpdateServerState(ss *ServerState)

	//选择服务器
	SelectServer(r *ReqSlbTask)

	//代替客户处理内容
	DoWork(r *ReqSlbTask)
}

//服务器状态，从健康检查机制获取
type ServerState struct {
	Ip string //服务器IP地址

	CupUtil int //cpu使用率

	IoWait   int   //磁盘IO快慢标志
	MemUtil  int   //内存使用率
	MemTotal int64 //内存总数，单位BYTE
	Down     bool  //服务器是否死掉
}

//最大权重限制为100
const maxWeight int = 100

type server struct {
	ip            string       //服务器IP
	port          int          //服务器工作端口
	weight        int          //服务器权重
	state         *ServerState //服务器状态
	specialty     taskProperty //服务器功能特长
	H264TaskSlice []int        //H264任务数组
	H265TaskSlice []int        //H265任务数组
	H264Capacity  int          //H264能力
	H265Capacity  int          //H265能力
}

type UserPolicy struct {
	UserID       string       //用户id，全局唯一
	Priority     int          //用户优先级
	Ip           serverIP     //给此用户分配的服务器IP地址
	ResponseChan *chan string //针对用户策略的回复通道。
}

type serverIP string
type userID string

//当前只支持轮询方式
type selectServerMode string

const (
	CHash      = selectServerMode("chash")
	RoundRobin = selectServerMode("roundrobin")
	Hash       = selectServerMode("hash")
)

type strategy struct {
	servers           map[serverIP]*server         //服务器集合
	usersPolicy       map[userID]*UserPolicy       //用户策略集合
	stateChannel      chan *ServerState            // 接收服务器状态通知的通道
	userPolicyChannel chan *UserPolicy             //接收用户策略通道
	slbReqChannel     chan *ReqSlbTask             //接收负载均衡请求通道
	configChannel     chan *config.Configuration   //接收配置文件通道
	quit              chan bool                    //服务退出通知通道
	prioriQueue       *priorityQueue               //优先级队列
	taskQueue         *taskCategoryQueue           //任务类别队列
	rr                map[taskProperty]*roundRobin //轮询调度
	myDBOperator      *(config.DBOperator)
	lock              sync.Mutex //在config文件发过来server信息的更新信息后，由于有可能正在处理查询服务器的请求。要加锁
}

func NewUserPolicy() *UserPolicy {
	defer utils.DealPanic()
	r := &UserPolicy{}
	chanStr := make(chan string, 1)
	r.ResponseChan = &chanStr
	return r
}

func NewStrategy() *strategy {
	defer utils.DealPanic()
	s := &strategy{}
	s.servers = make(map[serverIP]*server)
	s.usersPolicy = make(map[userID]*UserPolicy)
	s.stateChannel = make(chan *ServerState, 1)
	s.userPolicyChannel = make(chan *UserPolicy, 1)
	s.slbReqChannel = make(chan *ReqSlbTask, 1)
	s.configChannel = make(chan *config.Configuration, 1)
	s.quit = make(chan bool, 1)
	s.taskQueue = NewTaskCategoryList()
	s.prioriQueue = NewPriorityList(s.taskQueue)

	//往Policy里面填数据.
	s.initPolicy()

	// 没有config文件了，也不需要初始化initBackends
	//s.initBackends(cf)

	s.rr = make(map[taskProperty]*roundRobin) //轮询调度
	s.updateRR()

	s.taskQueue.strategy = s

	return s
}

//从数据库里面获取policy信息

func (s *strategy) initPolicy() {

	defer utils.DealPanic()

	s.myDBOperator = config.NewDBOperator()
	s.myDBOperator.InitSqlParam()
	err, result, dbString := s.myDBOperator.QueryPolicyTab()
	if nil == err && 0 == result {

		utils.Log.Debug("dbString =%s", dbString)
	}
	var ss [](config.PolicyInfo)

	json.Unmarshal([]byte(dbString), (&ss))

	utils.Log.Debug("dbString =%v", ss)

	for i := 0; i < len(ss); i++ {
		body := NewUserPolicy()
		body.UserID = ss[i].UserId
		temp, err := strconv.Atoi(ss[i].Priority)
		if nil == err {
			body.Priority = temp
		}
		body.Ip = (serverIP)(ss[i].Ip)
		s.usersPolicy[userID(body.UserID)] = body
	}
	utils.Log.Debug("len of s.usersPolicy  =%s", len(s.usersPolicy))
}

//初始化轮询策略
func (s *strategy) updateRR() {

	defer utils.DealPanic()

	var cpuServer []*server
	var gpuServer []*server
	s.lock.Lock()
	for _, v := range s.servers {
		if v.specialty == CpuPro {
			utils.Log.Debug("cpu: %+v", v)
			cpuServer = append(cpuServer, v)
		} else {
			gpuServer = append(gpuServer, v)
			utils.Log.Debug("gpu: %+v", v)
		}
	}
	s.rr[GpuPro] = NewRoundRobin(gpuServer)
	s.rr[CpuPro] = NewRoundRobin(cpuServer)
	s.lock.Unlock()

}

//初始化后端服务器
func (s *strategy) initBackends(cf *config.Configuration) {

	defer utils.DealPanic()
	utils.Log.Debug("%v", cf)
	for _, serverCfg := range cf.Backends {
		var stem server
		stem.ip = serverCfg.Host
		stem.specialty = taskProperty(strings.ToUpper(serverCfg.Specialty))
		stem.weight = serverCfg.Weight
		stem.state = &ServerState{
			Down: true,
		}
		s.servers[serverIP(stem.ip)] = &stem
		utils.Log.Debug("ip:%s,server:%v", stem.ip, stem)
	}

	for key, v := range s.servers {
		utils.Log.Debug("servers key:%s, value:%v", key, v)
	}
}

func (s *strategy) Run() {

	defer utils.DealPanic()
	go s.prioriQueue.run()
	s.taskQueue.run()

	for {
		select {
		case req := <-s.slbReqChannel:
			s.dealSlbReq(req)
		case up := <-s.userPolicyChannel:
			s.dealUpdateUserPolicy(up)
		case state := <-s.stateChannel:
			s.dealUpdateServerState(state)
		case state := <-s.configChannel:
			s.dealUpdateConfig(state)
		case <-s.quit:
			utils.Log.Debug("strategy run exit")
			return
		}
	}
}

func (s *strategy) Quit() {

	defer utils.DealPanic()
	s.quit <- true

	s.prioriQueue.quit()
	s.taskQueue.quit()
}

func (s *strategy) AddSlbReq(r *ReqSlbTask) {

	defer utils.DealPanic()
	r.TaskType = strings.ToUpper(r.TaskType)
	s.slbReqChannel <- r
}

func (s *strategy) UpdateUserPolicy(up *UserPolicy) {
	defer utils.DealPanic()
	s.userPolicyChannel <- up
}

func (s *strategy) UpdateConfig(up *config.Configuration) {
	defer utils.DealPanic()
	s.configChannel <- up
}

func (s *strategy) UpdateServerState(ss *ServerState) {
	defer utils.DealPanic()
	s.stateChannel <- ss
}

func (s *strategy) dealSlbReq(r *ReqSlbTask) {
	defer utils.DealPanic()
	up, ok := s.usersPolicy[userID(r.UserID)]

	if ok {
		if up.Ip == "" {
			r.priority = (priorityLevel)(up.Priority)
		} else {
			server := s.servers[up.Ip]
			//response := &responseSlbReq{}
			var strIpPort string
			if nil == server {
				//如果客户通过 curl -i -d '{"UserID":"123456","Priority":5,"Ip":"192.168.1.252"}' http://192.168.59.128:8081/yfy/user/policy
				//指定了一个无效ip， 那么当用户"123456"再来请求时  server := s.servers[up.Ip] 就是空
				//response.RetCode = retCodeFail
				strIpPort = ""
				utils.Log.Debug(" the userID ip(%s) did not in serverlist", up.Ip)
			} else {
				if server.state.Down {
					strIpPort = ""
					//strIpPort = server.ip + ":" + strconv.Itoa(server.port)
				} else {
					//response.RetCode = retCodeSuccess
					//response.IP = server.ip
					strIpPort = server.ip + ":" + strconv.Itoa(server.port)
				}
			}

			//responseToClient(r.ResponseChan, response)
			ipPortSendToClient(r.ResponseChan, strIpPort)
			return
		}
	}

	utils.Log.Debug(" the userID is did not in the usersPolicy so should add in the prioriQueue list ", r)
	s.prioriQueue.listAdd(r)
}

//返回slb获取的分配的服务器给客户端
func responseToClient(responseChan *chan string, response interface{}) {
	defer utils.DealPanic()
	buf, _ := json.Marshal(response)

	*responseChan <- string(buf)

	utils.Log.Debug(response)
}

//对于proxy类型的请求， 发送Ip字符串到engine 模块
func ipPortSendToClient(responseChan *chan string, IPString string) {
	defer utils.DealPanic()
	utils.Log.Debug("before  -> SelectServer ip port:%s", IPString)
	*responseChan <- IPString
	utils.Log.Debug("after -> SelectServer ip port:%s", IPString)
}

//处理用户策略设置
func (s *strategy) dealUpdateUserPolicy(up *UserPolicy) {
	defer utils.DealPanic()

	if "" == up.UserID {

		response := &responsePolicyReq{}

		response.RetCode = retCodeFail

		response.Result = "fail,the userId is null"
		responseToClient(up.ResponseChan, response)
		return
	}

	if "" != up.Ip {
		//正则匹配 1~255.0~255.0~255.0~255
		const lostPattern = "^(1\\d{2}|2[0-4]\\d|25[0-5]|[1-9]\\d|[1-9])\\." + "(1\\d{2}|2[0-4]\\d|25[0-5]|[1-9]\\d|\\d)\\." + "(1\\d{2}|2[0-4]\\d|25[0-5]|[1-9]\\d|\\d)\\." + "(1\\d{2}|2[0-4]\\d|25[0-5]|[1-9]\\d|\\d)$"
		reg, _ := regexp.Compile(lostPattern)
		if false == reg.MatchString(string(up.Ip)) {
			utils.Log.Debug("IP is not right: %s", string(up.Ip))
			response := &responsePolicyReq{}
			response.RetCode = retCodeFail
			response.Result = "fail,the ip style is not right"
			responseToClient(up.ResponseChan, response)
			return
		}
		//看看用户设置的IP是不是在服务器集合里面

		_, ok := s.servers[serverIP(up.Ip)]
		if !ok {
			utils.Log.Debug("the user  IP is not in the servers list: %s", string(up.Ip))
			response := &responsePolicyReq{}
			response.RetCode = retCodeFail
			response.Result = "fail,the user`s IP is not in the servers list"
			responseToClient(up.ResponseChan, response)
			return
		}
	} else {
		utils.Log.Debug("the user  IP null, so the Priority will be affect ", string(up.Ip))
	}

	s.usersPolicy[userID(up.UserID)] = up

	//添加到数据库里面

	s.myDBOperator.QueryAndInsertPolicyTab(up.UserID, up.Priority, string(up.Ip))

	for k, v := range s.usersPolicy {
		fmt.Println(k, v)
	}

	response := &responsePolicyReq{}
	response.RetCode = retCodeSuccess
	response.Result = "success"
	responseToClient(up.ResponseChan, response)
}

//处理config更新请求
func (s *strategy) dealUpdateConfig(up *config.Configuration) {

	defer utils.DealPanic()

	// 比对更新数据很繁琐，易出错
	//1 更新 strategy.servers           map[serverIP]*server //服务器集合

	for _, serverCfg := range up.Backends {

		if _, ok := s.servers[(serverIP)(serverCfg.Host)]; ok {

			//处理已经存在的服务器信息，更新信息。
			s.servers[(serverIP)(serverCfg.Host)].specialty = taskProperty(strings.ToUpper(serverCfg.Specialty))
			s.servers[(serverIP)(serverCfg.Host)].weight = serverCfg.Weight
			s.servers[(serverIP)(serverCfg.Host)].port = serverCfg.Serverport
			utils.Log.Debug("Update old server ip:%s", s.servers[serverIP(serverCfg.Host)].ip)

		} else {
			//处理新增的服务器信息,

			var stem server
			stem.ip = serverCfg.Host
			stem.port = serverCfg.Serverport
			stem.specialty = taskProperty(strings.ToUpper(serverCfg.Specialty))
			stem.weight = serverCfg.Weight

			stem.state = &ServerState{
				Down: true,
			}
			s.servers[serverIP(stem.ip)] = &stem

			utils.Log.Debug("dealUpdateConfig new add server ip:%s,server:%v", stem.ip, stem)

		}

	}

	//2处理数据库里面减少的数据项，只需把他们从Backends里去掉即可。
	for _, svr := range s.servers {
		var shouldDel bool
		shouldDel = true
		for _, serverCfg := range up.Backends {
			if svr.ip == serverCfg.Host {
				//engine切片中的IP在config中也存在，这种情况下，不能清除。
				shouldDel = false
				break
			}
		}
		//删除从数据库中清除掉的记录
		if true == shouldDel {
			delete(s.servers, (serverIP)(svr.ip))
		}
	}

	//3 更新 strategy.rr 里面的服务器集合
	//清空cpu gpu 两个切片里面的内容。
	s.rr[GpuPro].servers = nil
	s.rr[CpuPro].servers = nil
	s.updateRR()

}

//处理健康检查服务器，发送来的服务器信息
func (s *strategy) dealUpdateServerState(state *ServerState) {

	defer utils.DealPanic()
	fmt.Println("dealUpdateServerState")
	if v, ok := s.servers[serverIP(state.Ip)]; ok {
		v.state = state
		utils.Log.Debug("server:%v,state:%v", v, v.state)
	} else {
		utils.Log.Error("not exit server:%s", state.Ip)
	}
}

//确定某台机器进行服务
func (s *strategy) SelectServer(r *ReqSlbTask) {
	defer utils.DealPanic()

	//response := &responseSlbReq{}
	//response.RetCode = retCodeServerBusy
	var strIpPort string
	s.lock.Lock()
	v, ok := s.rr[taskProperty(r.TaskType)]
	if !ok {
		utils.Log.Error("req task type:%s no corresponding server list")
		//请求任务类型未对应服务器链表，则使用CPU类型的的链表
		//response.RetCode = retCodeNoTaskServer
		strIpPort = ""
	} else {
		ser, err := v.getBackendServer()
		if err != nil {

			utils.Log.Debug("SelectServer:%s", err.Error())
			strIpPort = ""

		} else {
			strIpPort = ser.ip + ":" + strconv.Itoa(ser.port)

		}
	}
	s.lock.Unlock()
	//responseToClient(r.ResponseChan, response)
	utils.Log.Debug("SelectServer ip port:%s", strIpPort)
	ipPortSendToClient(r.ResponseChan, strIpPort)

}

//确定某台机器进行服务
func (s *strategy) DoWork(r *ReqSlbTask) {
	defer utils.DealPanic()

	s.lock.Lock()
	var strIpPort string
	v, ok := s.rr[taskProperty(r.TaskType)]
	if !ok {
		utils.Log.Error("req task type:%s no corresponding server list")
		//请求任务类型未对应服务器链表，则使用CPU类型的的链表
		//l, ok := s.serverSpecialtyList[CpuPro]
	} else {
		ser, err := v.getBackendServer()
		if err != nil {
			utils.Log.Debug("1111111SelectServer:%s", err.Error())
		} else {
			strIpPort = ser.ip + ":" + strconv.Itoa(ser.port)
			utils.Log.Debug("2222222SelectServer:%s", strIpPort)
		}
	}
	ipPortSendToClient(r.ResponseChan, strIpPort)
	s.lock.Unlock()
}

package healthcheck_server

import (
	"common/config/goini"
	"common/utils"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	//"os"
	"strconv"
	"strings"
	"time"
)

// 要统计的服务器的状态信息
type ServerState struct {
	CupUtil  int
	IoWait   int
	MemUtil  int
	MemTotal int64
}

// udp的监听端口
type LisenPort struct {
	UdpPort string
}

// 监听任务 结构体集合，便于集中调用子接口。
type LisenTask struct {
	LisenPort
	ServerState
}

func checkError(err error) {
	if err != nil {
		utils.Log.Debug("Error: %s", err.Error())
	}
}

func (lisener *LisenTask) Run() {

	//读取端口号
	lisener.GetPortFromCfg()
	//启动监听
	lisener.LisenUDP()
}

func (Lisen *LisenPort) GetPortFromCfg() {
	defer utils.DealPanic()
	config_ptr := goini.Init("config.ini")
	Lisen.UdpPort = config_ptr.Read_string("listenPort", "listen_port", "80")
	utils.Log.Debug("Lisen.UdpPort = %s", Lisen.UdpPort)
}

func NewLisenTask() *LisenTask {
	return &LisenTask{
		LisenPort:   newLisenPort(),
		ServerState: newServer(),
	}
}

func newServer() ServerState {
	return ServerState{
		CupUtil:  0,
		MemUtil:  0,
		MemTotal: 0,
	}
}

func newLisenPort() LisenPort {
	return LisenPort{
		UdpPort: "8000",
	}
}

func (s *ServerState) readCpuFile() (rec bool, name string, user int64, nice int64, sys int64, idle int64, iowait int64, irq int64, softirq int64) {

	defer utils.DealPanic()
	stringSlice := make([]string, 10)
	dataSlice := make([]string, 10)
	data, err := ioutil.ReadFile("/proc/stat")

	if err != nil {
		fmt.Println("open file error:%s", err)
		return false, " ", 0, 0, 0, 0, 0, 0, 0
	}
	//utils.Log.Debug("Lisen.UdpPort","first time read /proc/stat data = ", string(data))
	stringSlice = strings.Split(string(data), "\n")
	//string_slice[0]=%d  cpu  3371 246 3152 1936419 6566 0 54 0 0 0
	utils.Log.Debug("first time string_slice[0]=%s ", stringSlice[0])
	dataSlice = strings.Fields(stringSlice[0])
	name = dataSlice[0]
	user, _ = strconv.ParseInt(dataSlice[1], 10, 64)
	nice, _ = strconv.ParseInt(dataSlice[2], 10, 64)
	sys, _ = strconv.ParseInt(dataSlice[3], 10, 64)
	idle, _ = strconv.ParseInt(dataSlice[4], 10, 64)
	iowait, _ = strconv.ParseInt(dataSlice[5], 10, 64)
	irq, _ = strconv.ParseInt(dataSlice[6], 10, 64)
	softirq, _ = strconv.ParseInt(dataSlice[7], 10, 64)
	return true, name, user, nice, sys, idle, iowait, irq, softirq
}

func (s *ServerState) getCpuinfo() {

	defer utils.DealPanic()
	var user, nice, sys, idle, iowait, irq, softirq int64
	var name string
	var rec bool
	var all1, all2, idle1, idle2, iowait1, iowait2 int64

	//第一次获取数据
	rec, name, user, nice, sys, idle, iowait, irq, softirq = s.readCpuFile()
	if false == rec {
		utils.Log.Error("first time read cpu file failed!")
		return
	}
	all1 = user + nice + sys + idle + iowait + irq + softirq
	idle1 = idle
	iowait1 = iowait
	utils.Log.Debug("first time name=%s,user= %v,nice= %v,sys=%v,idle=%v,iowait=%v,irq=%v softirq= %v ", name, user, nice, sys, idle, iowait, irq, softirq)

	//第二次获取数据
	time.Sleep(time.Millisecond * 500)
	rec, name, user, nice, sys, idle, iowait, irq, softirq = s.readCpuFile()
	if false == rec {
		utils.Log.Error("second time read cpu file failed!")
		return
	}
	all2 = user + nice + sys + idle + iowait + irq + softirq
	idle2 = idle

	iowait2 = iowait

	utils.Log.Debug("second time name=%s,user= %v,nice= %v,sys=%v,idle=%v,iowait=%v,irq= %v softirq= %v ", name, user, nice, sys, idle, iowait, irq, softirq)

	if 0 != all2-all1 {

		s.CupUtil = (int)(((all2 - idle2 - iowait2) - (all1 - idle1 - iowait1)) * 100 / (all2 - all1))
		s.IoWait = (int)((iowait2 - iowait1) * 100 / (all2 - all1))
	} else {
		s.CupUtil = 50
		s.IoWait = 0
	}
	utils.Log.Debug("s.CupUtil = %d,s.IoWait= %d  ", s.CupUtil, s.IoWait)
}

func (s *ServerState) getMeminfoValue(stringSlice []string, strValue string) string {

	defer utils.DealPanic()
	var rec string
	rec = " "
	for i := 0; i < len(stringSlice); i++ {
		//查看某一行是否包含特定字符串
		if strings.Contains(stringSlice[i], strValue) {
			//如果包含继续切片把字符串对应的值切出来
			dataSlice := make([]string, 10)
			dataSlice = strings.Fields(stringSlice[i])
			rec = dataSlice[1]
			utils.Log.Debug("getMeminfoValue rec =%s len = %d i=%d,stringSlice[i]=%s  ", rec, len(stringSlice), i, stringSlice[i])
			return rec
		}
	}
	return rec
}

func (s *ServerState) getMeminfo() {

	defer utils.DealPanic()
	var total, free, buffers, cached int64
	total = 0
	free = 0
	buffers = 0
	cached = 0
	stringSlice := make([]string, 10)
	str_memtotal := "MemTotal"
	str_memfree := "MemFree"
	str_buffers := "Buffers"
	str_cached := "Cached"

	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		utils.Log.Debug("open file error:%v", err)
		return
	}
	//utils.Log.Debug("second time read /proc/stat data =%s ", string(data))

	//按照"\n"把每行分别放到切片中
	stringSlice = strings.Split(string(data), "\n")

	//从切片中把数值分析出来

	total, _ = strconv.ParseInt(s.getMeminfoValue(stringSlice, str_memtotal), 10, 64)
	free, _ = strconv.ParseInt(s.getMeminfoValue(stringSlice, str_memfree), 10, 64)
	buffers, _ = strconv.ParseInt(s.getMeminfoValue(stringSlice, str_buffers), 10, 64)
	cached, _ = strconv.ParseInt(s.getMeminfoValue(stringSlice, str_cached), 10, 64)

	utils.Log.Debug("getMeminfo,total=%v,free=%v,buffers=%v,cached=%v", total, free, buffers, cached)

	//单位由kbyte
	s.MemTotal = total / 1024

	if 0 == total {

		s.MemUtil = 50

	} else {

		s.MemUtil = (int)((total - free - buffers - cached) * 100 / total)
	}
	utils.Log.Debug("MemTotal =%d, MemUtil=%d ", s.MemTotal, s.MemUtil)

}

func (s *LisenTask) LisenUDP() {

	defer utils.DealPanic()
	p := make([]byte, 2048)
	addrStr := ":" + s.LisenPort.UdpPort
	utils.Log.Debug("begin LisenUDP the lisen port is %s ", addrStr)
	udp_addr, err := net.ResolveUDPAddr("udp", addrStr)
	checkError(err)

	conn, err := net.ListenUDP("udp", udp_addr)
	defer conn.Close()
	checkError(err)

	for {
		_, remoteaddr, err := conn.ReadFromUDP(p)

		utils.Log.Debug("Read a message from %v %s n", remoteaddr, p)
		if err != nil {
			utils.Log.Debug("Some error %v", err)
			continue
		}
		go s.getServerState(conn, remoteaddr)
	}
}

func (lisen *LisenTask) getServerState(conn *net.UDPConn, addr *net.UDPAddr) {

	defer utils.DealPanic()
	lisen.getCpuinfo()
	lisen.getMeminfo()
	lisen.sendResponse(conn, addr)
}

func (s *ServerState) sendResponse(conn *net.UDPConn, addr *net.UDPAddr) {

	defer utils.DealPanic()
	//把数据以json的格式回应给发送者
	tmpJson := ServerState{
		CupUtil:  s.CupUtil,
		IoWait:   s.IoWait,
		MemUtil:  s.MemUtil,
		MemTotal: s.MemTotal,
	}

	jsonData, _ := json.Marshal(tmpJson)
	tmp_string := string(jsonData)
	_, err := conn.WriteToUDP([]byte(tmp_string), addr)
	if err != nil {
		utils.Log.Debug("Couldn’t send response %v", err)
	}
}

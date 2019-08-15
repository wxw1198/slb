package config

import (
	"bytes"
	"common/config/goini"
	"common/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//读取json格式的 配置文件
type sqlInfo struct {
	Name              string
	Healthcheckport   string
	Serverport        string
	Ip                string
	Weight            string
	Specialty         string
	HeartbeatInterval string
	RetryTime         string
}

type GlobConf struct {
	Timeout           int    //slb服务全局超时时间
	SlbStateInterface string //slb状态监听端口
}

type Backend struct {
	Name            string
	Host            string
	Healthcheckport int
	Serverport      int
	Specialty       string //此服务器特长点：CPU / GPU
	Weight          int
	Healthcheck     healthcheck
}

type healthcheck struct {
	HeartbeatInterval int //检查时间间隔，单位秒
	RetryNum          int //失败情况下，连续监测次数
}

type Configuration struct {
	Glob     GlobConf
	Backends []Backend
}

type CfgServer struct {
	Configuration
	myDBOperator   *DBOperator
	engineUrl      string
	healthcheckUrl string
}

//定义数据结构。
func NewCfgServer() *CfgServer {
	return &CfgServer{
		Configuration: newConfiguration(),
		myDBOperator:  NewDBOperator(),
	}
}

func newConfiguration() Configuration {
	return Configuration{
		Glob:     GlobConf{},
		Backends: make([]Backend, 1),
	}
}

func (s *CfgServer) LoadConfig() {

	//读取配置文件
	config_ptr := goini.Init("config.ini")
	s.Configuration.Glob.SlbStateInterface = config_ptr.Read_string("WEB", "SlbStateInterface", "")
	s.engineUrl = config_ptr.Read_string("PurposeUrl", "SlbEngineUrl", "")
	s.healthcheckUrl = config_ptr.Read_string("PurposeUrl", "SlbHealthycheckUrl", "")
	timeout := config_ptr.Read_string("WEB", "Timeout", "")
	temp, err := strconv.Atoi(timeout)
	if err != nil {
		utils.Log.Error("read Timeout from config.ini then strconv.Atoi failed err:%v", err)
		fmt.Println("read Timeout from config.ini then strconv.Atoi failed err:%v", err)
		return
	}
	s.Configuration.Glob.Timeout = temp
	utils.Log.Info("s.Glob.SlbStateInterface = %s,s.Glob.Timeout=%d", s.Glob.SlbStateInterface, s.Glob.Timeout)
	fmt.Println("s.Glob.SlbStateInterface = %v,s.Glob.Timeout=%v", s.Glob.SlbStateInterface, s.Glob.Timeout)
	fmt.Println("connect SQL info :successfull")

	//打开数据库
	if false == s.myDBOperator.InitSqlParam() {
		utils.Log.Info("connect to db failed")
		return
	}

}

func (s *CfgServer) Run() {

	for {
		//读取数据库并发送给engine和healthcheck模块。
		if false == s.myDBOperator.connectDB() {
			utils.Log.Info("connect to db failed")
			s.myDBOperator.closeDB()

			time.Sleep(10 * time.Second)
			continue
		}
		querySql := fmt.Sprintf("select name, healthcheckport,serverport, ip, weight, specialty, heartbeatInterval, retryTime from `slb_svr`;")
		fmt.Println("ReadDb cmd:", querySql)
		rows, err := s.myDBOperator.SlbDB.Query(querySql)
		if err != nil {
			fmt.Println("selectAllContent err:%v", err)
			s.myDBOperator.closeDB()

			time.Sleep(10 * time.Second)
			continue
		}
		jsonString := utils.RowsToJson(rows)
		rows.Close()

		var sqlInfoArray []sqlInfo
		json.Unmarshal([]byte(jsonString), (&sqlInfoArray))
		fmt.Println("Read Sql info jsonString = %s", jsonString)
		fmt.Println("Read Sql info jsonString = %v", sqlInfoArray)
		fmt.Println("Read Sql info len(sqlInfoArray) = %d", len(sqlInfoArray))

		//对比内存中数据看看数据库中数据是否改变
		tempSlice := make([]Backend, len(sqlInfoArray))
		s.ReadDbtoBackends(tempSlice, sqlInfoArray)
		if false == s.checkIfShoudSend(tempSlice) {
			utils.Log.Info("DB not change so should not send to engine or healthcheck")

			s.myDBOperator.closeDB()
			time.Sleep(time.Minute)
			continue
		} else {
			utils.Log.Info("DB changed so should  send to engine and  healthcheck")
		}
		s.Backends = nil
		s.Backends = tempSlice
		fmt.Println("Read Sql info len(sqlInfoArray) = %v", s)

		//把读出来的数据放到数据结构中，并发送出去
		utils.Log.Info("s.Configuration = %v", s.Configuration)
		// sent 数据给engine和health_check, 看门狗启动这两个模块后，不能立马发数据给他们，延时2秒，等engine和health_check的http模块完全准备好后再发送
		time.Sleep(2 * time.Second)
		s.SendOut(s.healthcheckUrl)
		s.SendOut(s.engineUrl)
		s.myDBOperator.closeDB()
		time.Sleep(time.Minute)
	}
}

func (s *CfgServer) ReadDbtoBackends(sqlInfoBackends []Backend, sqlInfoArray []sqlInfo) {

	for i := 0; i < len(sqlInfoArray); i++ {

		sqlInfoBackends[i].Name = sqlInfoArray[i].Name
		sqlInfoBackends[i].Host = sqlInfoArray[i].Ip

		temp, err := strconv.Atoi(sqlInfoArray[i].Healthcheckport)
		if err != nil {
			utils.Log.Error("read Healthcheckport  from config.ini then strconv.Atoi failed err:%v", err)
			fmt.Println("read Healthcheckport  from config.ini then strconv.Atoi failed err:%v", err)
			continue
		}
		sqlInfoBackends[i].Healthcheckport = temp

		temp, err = strconv.Atoi(sqlInfoArray[i].Serverport)
		if err != nil {
			utils.Log.Error("read Serverport  from config.ini then strconv.Atoi failed err:%v", err)
			fmt.Println("read Serverport  from config.ini then strconv.Atoi failed err:%v", err)
			continue
		}
		sqlInfoBackends[i].Serverport = temp

		sqlInfoBackends[i].Specialty = sqlInfoArray[i].Specialty

		temp, err = strconv.Atoi(sqlInfoArray[i].Weight)
		if err != nil {
			utils.Log.Error("read Weight from config.ini then strconv.Atoi failed err:%v", err)
			fmt.Println("read Weight from config.ini then strconv.Atoi failed err:%v", err)
			continue
		}
		sqlInfoBackends[i].Weight = temp

		temp, err = strconv.Atoi(sqlInfoArray[i].HeartbeatInterval)
		if err != nil {
			utils.Log.Error("read HeartbeatInterval from config.ini then strconv.Atoi failed err:%v", err)
			fmt.Println("read HeartbeatInterval from config.ini then strconv.Atoi failed err:%v", err)
			continue
		}
		sqlInfoBackends[i].Healthcheck.HeartbeatInterval = temp

		temp, err = strconv.Atoi(sqlInfoArray[i].RetryTime)
		if err != nil {
			utils.Log.Error("read RetryTime from config.ini then strconv.Atoi failed err:%v", err)
			fmt.Println("read RetryTime from config.ini then strconv.Atoi failed err:%v", err)
			continue
		}
		sqlInfoBackends[i].Healthcheck.RetryNum = temp
	}
}

func (s *CfgServer) checkIfShoudSend(sqlInfoArray []Backend) bool {

	if len(s.Backends) != len(sqlInfoArray) {
		return true
	}
	for i := 0; i < len(sqlInfoArray); i++ {
		if sqlInfoArray[i] != s.Backends[i] {
			return true
			break
		}
	}
	return false

}

func (s *CfgServer) SendOut(url string) {
	b, err := json.Marshal(s.Configuration)
	if err != nil {
		fmt.Println("json err:", err)
	}
	utils.Log.Debug("postbody= %s", string(b))
	body := bytes.NewBuffer([]byte(b))
	fmt.Println("send config info,body:%v ", body)
	utils.Log.Debug("send config info,body:%v ", body)

	postUrl := url
	utils.Log.Debug("send config info postUrl=%s", postUrl)
	req, err := http.NewRequest("POST", postUrl, body)
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		utils.Log.Debug("sendout error", err)
		fmt.Println("sendout error:", err)
		return
	}
	resp.Body.Close()
}

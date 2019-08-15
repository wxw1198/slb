package config

import (
	"common/config/goini"
	"common/utils"
	"database/sql"
	"encoding/json"
	"fmt"
)

//读取json格式的 配置文件
type PolicyInfo struct {
	UserId   string
	Priority string
	Ip       string
}
type DBOperator struct {
	MysqlParam string
	SlbDB      *sql.DB
}

func NewDBOperator() *DBOperator {
	return &DBOperator{}
}

func (s *DBOperator) InitSqlParam() bool {

	config_ptr := goini.Init("config.ini")
	// 链接数据库
	userName := config_ptr.Read_string("SQL", "userName", "")
	password := config_ptr.Read_string("SQL", "password", "")
	listenIp := config_ptr.Read_string("SQL", "listenIp", "")
	listenPort := config_ptr.Read_string("SQL", "listenPort", "")
	dbName := config_ptr.Read_string("SQL", "dbName", "")

	s.MysqlParam = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", userName, password, listenIp, listenPort, dbName)
	utils.Log.Info("SQL info :%s", s.MysqlParam)
	fmt.Println("SQL info :%s", s.MysqlParam)

	//打开数据库
	var err error
	s.SlbDB, err = sql.Open("mysql", s.MysqlParam)
	if err != nil { //连接成功 err一定是nil否则就是报错
		panic(err.Error())       //抛出异常
		fmt.Println(err.Error()) //仅仅是显示异常
		fmt.Println("connect SQL info : error %s", err.Error())
		return false
	}
	s.SlbDB.Close()
	return true

}

func (s *DBOperator) connectDB() bool {
	var err error
	utils.Log.Info("SQL info :%s", s.MysqlParam)
	s.SlbDB, err = sql.Open("mysql", s.MysqlParam)

	if err != nil { //连接成功 err一定是nil否则就是报错
		panic(err.Error())       //抛出异常
		fmt.Println(err.Error()) //仅仅是显示异常
		utils.Log.Info("connect SQL info : error %s", err.Error())
		return false
	}
	return true

}

func (s *DBOperator) closeDB() {
	s.SlbDB.Close()
}

func (s *DBOperator) QueryPolicyTab() (error, int, string) {
	s.connectDB()
	defer s.closeDB()

	updateSql := "select UserId , priority, Ip  from `slb_policy`"

	rows, err := s.SlbDB.Query(updateSql)

	if err != nil {
		utils.Log.Info("queryPolicyTab:" + err.Error())
		return err, 0, ""
	}

	str := utils.RowsToJson(rows)

	return nil, 0, str

}

func (s *DBOperator) QueryAndInsertPolicyTab(userid string, priority int, ip string) (error, int) {

	s.connectDB()
	defer s.closeDB()

	updateSql := "select UserId , priority, Ip  from `slb_policy`where UserID= ? "
	rows, err := s.SlbDB.Query(updateSql, userid)

	if err != nil {
		utils.Log.Info("QueryPolicyTab failed:" + err.Error())
		return err, 1
	}

	str := utils.RowsToJson(rows)
	var ss []PolicyInfo
	json.Unmarshal([]byte(str), (&ss))
	utils.Log.Info("insertAndUpdateSeatState  rows = %d \n", len(ss))
	if len(ss) == 0 {
		//should insert
		updateSql := "INSERT INTO `slb_policy` (UserId, priority, Ip) VALUES (?,?,?)"
		ret, err := s.SlbDB.Exec(updateSql, userid, priority, ip)

		if err != nil {
			utils.Log.Info("file programTbl insert:" + err.Error())
			utils.Log.Info("err =%v", ret)
			return err, 1
		}
		return err, 0

	} else {

		//should update
		var ret sql.Result
		var err error

		updateSql := "UPDATE `slb_policy` SET priority = ? ,Ip = ?  WHERE UserId = ?"
		ret, err = s.SlbDB.Exec(updateSql, priority, ip, userid)

		if err != nil {
			utils.Log.Info("file programTbl insert:" + err.Error())
			utils.Log.Info("err =%v", ret)
			return err, 1
		}
		utils.Log.Info("QueryPolicyTab  update  an old  line\n")
		return err, 0
	}
}

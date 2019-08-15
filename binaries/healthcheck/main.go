package main

import (
	"common/compile"
	"flag"
	"fmt"
	"slb/healthcheck"
)

var ver string = "1.0.0"

func main() {

	// ./main -V  查看程序版本号和编译时间
	checkVer := flag.Bool("V", false, "is ok")
	flag.Parse()
	if *checkVer {

		verStr := "ver: " + ver + "\r\n"
		fmt.Println(verStr + compile.BuildTime())
		return
	}

	var sender *healthcheck.CheckTask

	//新建立一个LisenTask，在LisenTask里面启动任务
	sender = healthcheck.NewCheckTask()
	sender.SetUp()
	sender.StartCheckingHealth()
	//utils.Log.Error("CheckTask %v", sender)
	//启动定时查询函数

}

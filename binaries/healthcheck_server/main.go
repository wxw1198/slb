package main

import (
	"common/compile"
	"flag"
	"fmt"
	"slb/healthcheck_server"
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

	var lisener *healthcheck_server.LisenTask
	//新建立一个lisenTask，在lisenTask里面启动任务
	lisener = healthcheck_server.NewLisenTask()
	//启动监听
	lisener.Run()
}

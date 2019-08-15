package main

import (
	"common/compile"
	"flag"
	"fmt"
	"slb/engine"
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
	slb := engine.NewSlb()
	slb.Run()
}

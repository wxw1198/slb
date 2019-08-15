package healthcheck_server

import (
	"testing"
	//
)

func TestStartLisening(t *testing.T) {

	var lisener *LisenTask
	//新建立一个lisenTask，在lisenTask里面启动任务
	lisener = NewLisenTask()
	//启动监听
	lisener.Run()
}

package healthcheck

import (
	"testing"
	//
)

func TestStartLisening(t *testing.T) {

	var sender *CheckTask

	//新建立一个CheckTask，在CheckTask里面启动任务
	sender = NewCheckTask()
	sender.SetUp()
	sender.StartCheckingHealth()

}

package strategy

//请求的模式
type reqMode string

const (
	SelectServer = reqMode("selectServer")
	DoWork       = reqMode("doWork")
)

//定义返回错误码
type retCode int

const (
	retCodeSuccess      = retCode(0) //成功
	retCodeFail         = retCode(1)
	retCodeNoTaskServer = retCode(2) //没有对应此任务类型的服务器
	retCodeServerBusy   = retCode(3) //服务器忙
)

//针对ReqSlbTask的回复
type responseSlbReq struct {
	RetCode retCode //成功此值是0，失败非0
	IP      string  //成功返回正确IP地址，失败此值无意义
}

//针对ReqPolicyTask的回复
type responsePolicyReq struct {
	RetCode retCode //成功此值是0，失败非0
	Result  string  //结果描述
}

type priorityLevel int

//0-9十个级别
const (
	priorityLeve0 priorityLevel = iota
	priorityLeve1
	priorityLeve2
	priorityLeve3
	priorityLeve4
	priorityLeve5
	priorityLeve6
	priorityLeve7
	priorityLeve8
	priorityLeve9
)

type SyncNoteType int

const (
	SNTHeartbeat SyncNoteType = iota
	SNTDesync
	SNTConfigUpdate
	SNTHealthcheck
	SNTOverride
)

//请求SLB服务器时，客户端带的body参数
type ReqSlbTask struct {
	UserID       string //用户ID，如果非特殊用户可以不设置此值。
	SessonID     string //会话ID
	TaskType     string        //任务类型
	priority     priorityLevel //用户优先级,提前被设置的
	ReqMode      reqMode       //请求模式，1 ”selectServer“ slb返回选择的机器 2 ”doWork“ slb直接帮助处理此请求;默认为1
	ResponseChan *chan string  //针对每个请求的同步回复通道
}

func NewReqSlbTask() *ReqSlbTask {
	r := &ReqSlbTask{}
	chanStr := make(chan string, 1)
	r.ResponseChan = &chanStr
	return r
}

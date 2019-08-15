package strategy

//此队列的作用优先级别控制，如果不需要关注优先级，可以不要此模块
import (
	"common/utils"
	"container/list"
	"sync"
	"time"
)

const MAX_PRIORITY_NUM = 10

//不同级别优先级的任务放入到不同级别的都列
type priorityQueue struct {
	lock       sync.Mutex
	lists      [MAX_PRIORITY_NUM]*list.List //0-9个链表，表明优先级为0-9,0优先级最低，9优先级最高
	taskQueues *taskCategoryQueue           //多类型任务队列
	quitChan   chan bool
}

func NewPriorityList(task *taskCategoryQueue) *priorityQueue {
	p := &priorityQueue{}
	i := 0
	for _ = range p.lists {
		p.lists[i] = list.New()
		i++
	}

	p.quitChan = make(chan bool, 1)

	p.taskQueues = task
	return p
}

func (l *priorityQueue) quit() {
	l.quitChan <- true
}

//当直播任务完成后，会自动调用remove移除链表
func (l *priorityQueue) listAdd(task *ReqSlbTask) {
	l.lock.Lock()
	defer l.lock.Unlock()
	priorityQueue := l.lists[task.priority]

	utils.Log.Debug("task.priority:%d", task.priority)
	priorityQueue.PushBack(task)
}

func (l *priorityQueue) listRemove(e *list.Element) {
	l.lock.Lock()
	defer l.lock.Unlock()

	task := e.Value.(*ReqSlbTask)

	priorityQueue := l.lists[task.priority]

	priorityQueue.Remove(e)
}

//从优先级队列中取出任务，分发到任务属性队列(此优先级是否有必要，后续视实际情况)
func (q *priorityQueue) run() {
	//每次循环最多处理个数，防止此协程持续运行时间过久
	const maxDealReqNum int = 1024
	dealNum := 0

	timeout := time.NewTicker(time.Millisecond * 10)

	for {
		select {
		case <-timeout.C:
			dealNum = 0
			q.modifyPriority()

			listNum := len(q.lists)

			for listNum >= 1 && dealNum < maxDealReqNum {

				singleList := q.lists[listNum-1]

				for singleList.Len() > 0 && dealNum < maxDealReqNum {

					dealNum++
					e := singleList.Front()
					if req, ok := e.Value.(*ReqSlbTask); ok {
						utils.Log.Debug("doReq priorityQueue: check and move req form priorityQueue to taskQueue: Prioritylist has %d lists for different priority  , now is deal with Priority[%d] list. (now is dealing) ", len(q.lists), listNum)
						utils.Log.Debug("doReq priorityQueue: the Priority[%d] list = has %d reqtasks  ", listNum, singleList.Len())
						q.taskQueues.listAdd(req)

						q.lock.Lock()
						singleList.Remove(e)
						q.lock.Unlock()

					}
				}

				listNum--
			}
		case <-q.quitChan:
			return
		}
	}
}

//优先级低的请求，如果长期不能被服务，调整其优先级别
func (l *priorityQueue) modifyPriority() {

}

package strategy

//此队列的作用：分类缓存
import (
	"common/utils"
	"container/list"
	"reflect"
	"sync"
	"time"
)

type taskProperty string

const (
	GpuPro = taskProperty("GPU")
	CpuPro = taskProperty("CPU")
)

//不同级别优先级的任务放入到不同级别的都列
type taskCategoryQueue struct {
	lock     sync.Mutex
	taskList map[taskProperty]*list.List //GPU/CPU两个任务类型链表
	quits    map[taskProperty]chan bool
	strategy StrategyInterface //服务器策略
}

func NewTaskCategoryList() *taskCategoryQueue {

	defer utils.DealPanic()

	t := &taskCategoryQueue{}
	t.taskList = make(map[taskProperty]*list.List)
	t.taskList[GpuPro] = list.New()
	t.taskList[CpuPro] = list.New()

	t.quits = make(map[taskProperty]chan bool)
	t.quits[GpuPro] = make(chan bool, 1)
	t.quits[CpuPro] = make(chan bool, 1)

	return t
}

func (t *taskCategoryQueue) quit() {
	defer utils.DealPanic()
	for k, _ := range t.quits {
		t.quits[k] <- true
	}
}

//任务分类链表
func (t *taskCategoryQueue) listAdd(task *ReqSlbTask) {

	defer utils.DealPanic()
	t.lock.Lock()
	defer t.lock.Unlock()

	q, ok := t.taskList[taskProperty(task.TaskType)]
	if !ok {
		utils.Log.Debug("list add err:%s", task.TaskType)
		return
	}

	utils.Log.Debug("to task queue,type:%s", task.TaskType)
	q.PushBack(task)
}

func (t *taskCategoryQueue) listRemove(e *list.Element) {

	defer utils.DealPanic()
	t.lock.Lock()
	defer t.lock.Unlock()

	task := e.Value.(*ReqSlbTask)

	q, ok := t.taskList[taskProperty(task.TaskType)]
	if !ok {
		utils.Log.Debug("listRemove err:%s", task.TaskType)
		return
	}

	q.Remove(e)
}

//从不同任务队列中获取任务，触发选择主机流程
func (t *taskCategoryQueue) run() {

	defer utils.DealPanic()
	//每次循环最多处理个数，防止此协程持续运行时间过久
	const maxDealReqNum int = 1024

	for key, v := range t.taskList {

		go func(taskQ *list.List, key taskProperty) {
			//缩小变量有效空间
			var dealNum int = 0
			timeout := time.NewTicker(time.Millisecond * 10)

			for {
				select {
				case <-timeout.C:

					dealNum = 0
					for taskQ.Len() > 0 && dealNum < maxDealReqNum {
						dealNum++
						e := taskQ.Front()
						task, ok := e.Value.(*ReqSlbTask)
						if !ok {
							utils.Log.Error("doReq taskQueue:list element not ReqSlbTask, type:%v", reflect.TypeOf(e))
						} else {
							utils.Log.Debug("doReq taskQueue: taskQueue len is %d, now is do with: %v ", taskQ.Len(), task)

							if SelectServer == task.ReqMode {
								t.strategy.SelectServer(task)
							} else if DoWork == task.ReqMode {
								t.strategy.DoWork(task)

							}

							t.listRemove(e)
						}
					}
				case <-t.quits[key]:
					utils.Log.Debug("task type %s run exit", string(key))
					return
				}
			}
		}(v, key)
	}
}

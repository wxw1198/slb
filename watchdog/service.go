// Copyright 2012 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software

// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Author: jsing@google.com (Joel Sing)

package watchdog

import (
	"common/utils"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

const prioProcess = 0 //线程优先级

// Service contains the data needed to manage a service.
type Service struct {
	name   string
	binary string   //看门狗要看守的应用程序的存放路径+程序名： /mnt/hgfs/svn_2/go/src/slb/binaries/healthcheck/slb_healthcheck
	path   string   //看门狗要看守的应用程序的工作路径： /mnt/hgfs/svn_2/go/src/slb/binaries/healthcheck/
	args   []string //看门狗要看守的应用程序的参数信息，暂不需要。

	priority int //线程优先级 可配置

	dependencies map[string]*Service // 某个server执行时的依赖模块 开始进程时要先执行被依赖的模块，比如healthcheck 依赖 engine模块那么启动时要先启动engine，再启动healthcheck
	dependents   map[string]*Service // 某个server被谁依赖. 关闭进程时，要查看进程被谁依赖，要先关闭它，比如healthcheck依赖engine模块（engine被healthcheck依赖）
	//那么关闭时要先关闭healthcheck，再关闭engine
	termTimeout time.Duration //某次启动失败，重新启动模块时的时间间隔。

	lock    sync.Mutex
	process *os.Process //某个模块对应的进程指针。

	done chan bool //shutdown 和down配合使用  例如当开门狗模块收到SIGINT信号会强制停止看门狗所看守的各个子模块，在停止子模块时，正常
	//情况下，只要各个子模块调用svc.signal(syscall.SIGTERM)杀死自己即可，没有必要同时使用 done 和 shutdown两个通道。但是假如healthcheck依赖engine模块
	//正在启动，而且此时 healthcheck 模块正在等待engine模块执行。此时看门狗收到了SIGQUIT要求停止其所看守的程序。看门狗重新开辟协程要求healthcheck执行终止自己的命令。
	//如果只是执行stop()函数。healthcheck 向自己发送 svc.signal(syscall.SIGTERM)，而healthcheck本身并未启动，是无效的。过一会 healthcheck启动程序执行时，可能还会
	//再起来。造成 healthcheck无法关闭的情况。在healthcheck执行stop()时 发送shutdown到通道。  healthcheck启动协程收到shutdown后绕过启动函数，不再启动。就好了。
	shutdown chan bool //shutdown 和down配合使用
	started  chan bool // 控制依赖模块和被依赖模块的执行先后顺序的通道（比如healthcheck 依赖 engine模块），只要被依赖的模块engine的started通道是ture后，才启动healthcheck模块
	stopped  chan bool // 控制依赖模块和被依赖模块的结束先后顺序的通道（比如healthcheck 依赖 engine模块），只有依赖别人的模块healthcheck的stoped通道是ture后，才停止被依赖的模块engine

	failures uint64
	restarts uint64

	lastFailure time.Time
	lastRestart time.Time
}

// newService returns an initialised service.
func newService(name, binary string) *Service {
	return &Service{
		name:         name,
		binary:       binary,
		args:         make([]string, 0),
		dependencies: make(map[string]*Service),
		dependents:   make(map[string]*Service),

		done:     make(chan bool),
		shutdown: make(chan bool, 1),
		started:  make(chan bool, 1),
		stopped:  make(chan bool, 1),

		termTimeout: 5 * time.Second,
	}
}

// AddDependency registers a dependency for this service.
func (svc *Service) AddDependency(name string) {
	svc.dependencies[name] = nil
}

// AddArgs adds the given string as arguments.
func (svc *Service) AddArgs(args string) {
	svc.args = strings.Fields(args)
}

// SetPriority sets the process priority for a service.
func (svc *Service) SetPriority(priority int) error {
	if priority < -20 || priority > 19 {
		return fmt.Errorf("Invalid priority %d - must be between -20 and 19", priority)
	}
	svc.priority = priority
	return nil
}

// SetTermTimeout sets the termination timeout for a service.
func (svc *Service) SetTermTimeout(tt time.Duration) {
	svc.termTimeout = tt
}

// 路径设进去，作为程序启动的参数
func (svc *Service) SetWorkPath(path string) {
	svc.path = path
}

// run runs a service and restarts it upon termination, unless a shutdown
// notification has been received.
func (svc *Service) run() {

	// Wait for dependencies to start.
	for _, dep := range svc.dependencies {
		utils.Log.Debug("Service %s waiting for %s to start", svc.name, dep.name)
		select {
		case started := <-dep.started:
			dep.started <- started
		case <-svc.shutdown:
			goto done
		}
	}

	for {
		if svc.failures > 0 {
			delay := time.Duration(svc.failures) * restartBackoff
			if delay > restartBackoffMax {
				delay = restartBackoffMax
			}
			utils.Log.Debug("Service %s has failed %d times - delaying %s before restart",
				svc.name, svc.failures, delay)

			select {
			case <-time.After(delay):
			case <-svc.shutdown:
				goto done
			}
		}

		svc.restarts++
		svc.lastRestart = time.Now()
		svc.runOnce()

		select {
		case <-time.After(restartDelay):
		case <-svc.shutdown:
			goto done
		}
	}
done:
	svc.done <- true
}

// runOnce runs a service once, returning once an error occurs or the process
// has exited.
func (svc *Service) runOnce() {

	args := make([]string, len(svc.args)+1)

	args[0] = "slb_" + svc.name
	copy(args[1:], svc.args)
	fmt.Println("runOnce args[0]=%s ", args[0])
	null, err := os.Open(os.DevNull)
	if err != nil {
		utils.Log.Debug("Service %s - failed to open %s: %v", svc.name, os.DevNull, err)
		fmt.Println("Service %s - failed to open %s: %v", svc.name, os.DevNull, err)
		return

	}

	_, pw, err := os.Pipe()
	if err != nil {
		utils.Log.Debug("Service %s - failed to create pipes: %v", svc.name, err)
		fmt.Println("Service %s - failed to create pipes: %v", svc.name, err)
		null.Close()
		return
	}

	fmt.Println("runOnce svc.path=%s ", svc.path)
	attr := &os.ProcAttr{
		Dir: svc.path,
		//Files: []*os.File{null, pw, pw},
		Files: []*os.File{null, null, null},
		Sys: &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: 0,
				Gid: 0,
			},
			Setpgid: true,
		},
	}

	fmt.Println("Starting service.name %s...", svc.name)
	fmt.Println("Starting service.path %s...", svc.path)
	fmt.Println("Starting svc.binary %v...", svc.binary)
	fmt.Println("Starting args %v...", args)
	fmt.Println("Starting attr %v...", attr)

	proc, err := os.StartProcess(svc.binary, args, attr)
	if err != nil {
		utils.Log.Debug("Service %s failed to start: %v", svc.name, err)
		fmt.Println("Service %s failed to start: %v", svc.name, err)
		svc.lastFailure = time.Now()
		svc.failures++
		null.Close()
		pw.Close()
		return
	}
	null.Close()
	pw.Close()
	svc.lock.Lock()
	svc.process = proc
	svc.lock.Unlock()

	if _, _, err := syscall.Syscall(syscall.SYS_SETPRIORITY, uintptr(prioProcess), uintptr(proc.Pid), uintptr(svc.priority)); err != 0 {
		utils.Log.Debug("Failed to set priority to %d for service %s: %v", svc.priority, svc.name, err)
	}

	select {
	case svc.started <- true:
	default:
	}

	state, err := svc.process.Wait()
	if err != nil {
		utils.Log.Debug("Service %s wait failed with %v", svc.name, err)
		svc.lastFailure = time.Now()
		svc.failures++
		return
	}
	if !state.Success() {
		utils.Log.Debug("Service %s exited with %v", svc.name, state)
		svc.lastFailure = time.Now()
		svc.failures++
		return
	}
	// TODO(jsing): Reset failures after process has been running for some
	// given duration, so that failures with large intervals do not result
	// in backoff. However, we also want to count the total number of
	// failures and export it for monitoring purposes.
	svc.failures = 0
	utils.Log.Debug("Service %s exited normally.", svc.name)
}

// signal sends a signal to the service.
func (svc *Service) signal(sig os.Signal) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.process == nil {
		return nil
	}
	return svc.process.Signal(sig)
}

// stop stops a running service.
func (svc *Service) stop() {
	// TODO(jsing): Check if it is actually running?
	utils.Log.Debug("Stopping service %s...", svc.name)

	// Wait for dependents to shutdown.
	for _, dep := range svc.dependents {
		utils.Log.Debug("Service %s waiting for %s to stop", svc.name, dep.name)
		stopped := <-dep.stopped
		dep.stopped <- stopped
	}

	svc.shutdown <- true
	svc.signal(syscall.SIGTERM)
	select {
	case <-svc.done:
	case <-time.After(svc.termTimeout):
		svc.signal(syscall.SIGKILL)
		<-svc.done
	}
	utils.Log.Debug("Service %s stopped", svc.name)
	svc.stopped <- true
}

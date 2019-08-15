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
	"os/signal"
	"syscall"
	"time"
	//log "github.com/golang/glog"
	conf "github.com/dlintw/goconf"
)

var signalNames = map[syscall.Signal]string{
	syscall.SIGINT:  "SIGINT",
	syscall.SIGQUIT: "SIGQUIT",
	syscall.SIGTERM: "SIGTERM",
}
var restartBackoff = 5 * time.Second
var restartBackoffMax = 60 * time.Second
var restartDelay = 2 * time.Second

// Watchdog contains the data needed to run a watchdog.
type Watchdog struct {
	services map[string]*Service
	shutdown chan bool
}

// NewWatchdog returns an initialised watchdog.
func NewWatchdog() *Watchdog {
	return &Watchdog{
		services: make(map[string]*Service),
		shutdown: make(chan bool),
	}
}

// signalName returns a string containing the standard name for a given signal.
func signalName(s syscall.Signal) string {
	if name, ok := signalNames[s]; ok {
		return name
	}
	return fmt.Sprintf("SIG %d", s)
}

// cfgOpt returns the configuration option from the specified section. If the
// option does not exist an empty string is returned.
func (w *Watchdog) cfgOpt(cfg *conf.ConfigFile, section, option string) string {
	if !cfg.HasOption(section, option) {
		return ""
	}
	s, err := cfg.GetString(section, option)
	if err != nil {
		utils.Log.Debug("Failed to get %s for %s: %v", option, section, err)
	}
	return s
}

// svcOpt returns the specified configuration option for a service.
func (w *Watchdog) svcOpt(cfg *conf.ConfigFile, service, option string, required bool) string {
	// TODO(jsing): Add support for defaults.
	opt := w.cfgOpt(cfg, service, option)
	if opt == "" && required {
		utils.Log.Debug("Service %s has missing %s option", service, option)
	}
	return opt
}

func (w *Watchdog) LoadCfg() {

	cfg, _ := conf.ReadConfigFile("./watchdog.cfg")
	utils.Log.Debug("begin watchdog")

	for _, name := range cfg.GetSections() {
		if name == "default" {
			continue
		}
		fmt.Println("watchdog name=%v", name)
		binary := w.svcOpt(cfg, name, "binary", true)
		args := w.svcOpt(cfg, name, "args", false)
		fmt.Println("watchdog binary=%v", binary)
		svc, err := w.AddService(name, binary)
		if err != nil {
			utils.Log.Debug("Failed to add service %q: %v", name, err)
		}
		svc.AddArgs(args)
		if dep := w.svcOpt(cfg, name, "dependency", false); dep != "" {
			svc.AddDependency(dep)
		}
		//添加子模块的工作路径
		if opt := w.svcOpt(cfg, name, "workpath", true); opt != "" {
			svc.SetWorkPath(opt)
		}
	}

}

func (w *Watchdog) ShutdownHandler() {
	sigc := make(chan os.Signal, 3)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		for s := range sigc {
			name := s.String()
			if sig, ok := s.(syscall.Signal); ok {
				name = signalName(sig)
			}
			fmt.Println("Received %v, initiating shutdown...", name)
			w.Shutdown()
		}
	}()
}

// Shutdown requests the watchdog to shutdown.
func (w *Watchdog) Shutdown() {
	select {
	case w.shutdown <- true:
	default:
	}
}

// AddService adds a service that is to be run by the watchdog.
func (w *Watchdog) AddService(name, binary string) (*Service, error) {
	if _, ok := w.services[name]; ok {
		return nil, fmt.Errorf("Service %q already exists", name)
	}

	svc := newService(name, binary)
	w.services[name] = svc

	return svc, nil
}

// Walk takes the watchdog component for a walk so that it can run the
// configured services.
func (w *Watchdog) Walk() {
	utils.Log.Debug("Seesaw watchdog starting...")
	fmt.Println("Seesaw watchdog starting...")
	w.mapDependencies()

	for _, svc := range w.services {
		go svc.run()
	}
	<-w.shutdown
	for _, svc := range w.services {
		go svc.stop()
	}
	for _, svc := range w.services {
		stopped := <-svc.stopped
		svc.stopped <- stopped
	}
}

// mapDependencies maps service dependency names to configured services.
func (w *Watchdog) mapDependencies() {
	for name := range w.services {
		svc := w.services[name]
		for depName := range svc.dependencies {
			dep, ok := w.services[depName]
			if !ok {
				utils.Log.Debug("Failed to find dependency %q for service %q", depName, name)
			}
			svc.dependencies[depName] = dep
			dep.dependents[svc.name] = svc
		}
	}
}

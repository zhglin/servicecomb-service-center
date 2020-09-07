/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package grace

import (
	"flag"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	PreSignal = iota
	PostSignal
)

//https://www.lmlphp.com/user/1234/article/item/29949/  Golang中的优雅重启
var (
	isFork         bool //标记是否是子进程
	filesOrder     string
	files          []*os.File     // listener的链接，被新进程继承
	filesOffsetMap map[string]int // listener的链接 索引

	registerSignals []os.Signal                    // 监听的信号
	SignalHooks     map[int]map[os.Signal][]func() // 信号的前置，后置的处理函数
	graceMux        sync.Mutex
	forked          bool // 标记开始复制
)

func init() {
	flag.BoolVar(&isFork, "fork", false, "listen on open fd (after forking)")
	flag.StringVar(&filesOrder, "filesorder", "", "previous initialization FDs order")

	registerSignals = []os.Signal{
		syscall.SIGHUP,  //用户终端连接(正常或非正常)结束时发出, 通常是在终端的控制进程结束时, 通知同一session内的各个作业, 这时它们与控制终端不再关联。
		syscall.SIGINT,  //程序终止(interrupt)信号, 在用户键入INTR字符(通常是Ctrl+C)时发出，用于通知前台进程组终止进程
		syscall.SIGKILL, //用来立即结束程序的运行. 本信号不能被阻塞、处理和忽略。
		syscall.SIGTERM, //程序结束(terminate)信号, 与SIGKILL不同的是该信号可以被阻塞和处理。通常用来要求程序自己正常退出，shell命令kill缺省产生这个信号
	}
	filesOffsetMap = make(map[string]int)

	// 监听信号的前置后置处理函数
	SignalHooks = map[int]map[os.Signal][]func(){
		PreSignal:  {},
		PostSignal: {},
	}
	for _, sig := range registerSignals {
		SignalHooks[PreSignal][sig] = []func(){}
		SignalHooks[PostSignal][sig] = []func(){}
	}

	go handleSignals()
}

func ParseCommandLine() {
	if !flag.Parsed() {
		flag.Parse()
	}
}

func Before(f func()) {
	RegisterSignalHook(PreSignal, f, syscall.SIGHUP)
}

func After(f func()) {
	RegisterSignalHook(PostSignal, f, registerSignals[1:]...)
}

// 提供的注册函数
func RegisterSignalHook(phase int, f func(), sigs ...os.Signal) {
	for s := range SignalHooks[phase] {
		for _, sig := range sigs {
			if s == sig {
				SignalHooks[phase][sig] = append(SignalHooks[phase][sig], f)
			}
		}
	}
}

// 注册需要被子进程继承的文件描述符
func RegisterFiles(name string, f *os.File) {
	if f == nil {
		return
	}
	graceMux.Lock()
	filesOffsetMap[name] = len(files)
	files = append(files, f)
	graceMux.Unlock()
}

// 执行信号的前置或后置函数
func fireSignalHook(ppFlag int, sig os.Signal) {
	if _, notSet := SignalHooks[ppFlag][sig]; !notSet {
		return
	}
	for _, f := range SignalHooks[ppFlag][sig] {
		f()
	}
}

// 处理信号
func handleSignals() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, registerSignals...)

	for sig := range sigCh {
		fireSignalHook(PreSignal, sig)
		switch sig {
		case syscall.SIGHUP: // 启动后关闭终端 damon
			log.Debugf("received signal '%v', now forking", sig)
			err := fork()
			if err != nil {
				log.Errorf(err, "fork a process failed")
			}
		}
		fireSignalHook(PostSignal, sig)
	}
}

// 复制已启动的进程
func fork() (err error) {
	graceMux.Lock()
	defer graceMux.Unlock()
	if forked {
		return
	}
	forked = true

	var orderArgs = make([]string, len(filesOffsetMap))
	for name, i := range filesOffsetMap {
		orderArgs[i] = name
	}

	// add fork and file descriptions order flags
	args := append(parseCommandLine(), "-fork") // 标记isFork=true
	if len(filesOffsetMap) > 0 {
		args = append(args, fmt.Sprintf(`-filesorder=%s`, strings.Join(orderArgs, ",")))
	}

	// 执行
	if err = newCommand(args...); err != nil {
		log.Errorf(err, "fork a process failed, %v", args)
		return
	}
	log.Warnf("fork process %v", args)
	return
}

// 解析当前进程的命令行参数，第一个是程序名
func parseCommandLine() (args []string) {
	if len(os.Args) <= 1 {
		return
	}
	// ignore process path
	for _, arg := range os.Args[1:] {
		if arg == "-fork" { // 去掉后面的-fork -filesorder
			// ignore fork flags
			break
		}
		args = append(args, arg)
	}
	return
}

// 执行命令
func newCommand(args ...string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = files // 指定额外被新进程继承的已打开文件流
	return cmd.Start()
}

// 是否是子进程
func IsFork() bool {
	return isFork
}

func ExtraFileOrder(name string) int {
	if len(filesOrder) == 0 {
		return -1
	}
	orders := strings.Split(filesOrder, ",")
	for i, f := range orders {
		if f == name {
			return i
		}
	}
	return -1
}

// 复制完成后关闭父进程
func Done() error {
	// 不是子进程
	if !IsFork() {
		return nil
	}

	ppid := os.Getppid()
	process, err := os.FindProcess(ppid)
	if err != nil {
		return err
	}
	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		return err
	}
	return nil
}

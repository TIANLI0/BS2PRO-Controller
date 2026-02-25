package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var app *CoreApp

	defer func() {
		if r := recover(); r != nil {
			capturePanic(app, "main", r)

			if app != nil {
				func() {
					defer func() {
						if stopPanic := recover(); stopPanic != nil {
							capturePanic(app, "main.Stop", stopPanic)
						}
					}()
					app.Stop()
				}()
			}

			os.Exit(1)
		}
	}()

	// 检测命令行参数
	debugMode := false
	isAutoStart := false

	for _, arg := range os.Args {
		switch arg {
		case "--debug", "/debug", "-debug":
			debugMode = true
		case "--autostart", "/autostart", "-autostart":
			isAutoStart = true
		}
	}

	// 创建核心应用
	app = NewCoreApp(debugMode, isAutoStart)

	// 启动应用
	if err := app.Start(); err != nil {
		panic(fmt.Sprintf("启动核心服务失败: %v", err))
	}

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		app.logInfo("收到系统退出信号")
	case <-app.quitChan:
		app.logInfo("收到应用退出请求")
	}

	app.Stop()
}

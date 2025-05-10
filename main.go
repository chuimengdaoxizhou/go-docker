package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

// usage 定义了应用的使用说明
const usage = `my docker`

func main() {
	// 创建一个新的CLI应用实例
	app := cli.NewApp()
	// 设置应用的名称
	app.Name = "mydocker"
	// 设置应用的使用说明
	app.Usage = usage

	// 定义应用支持的命令
	app.Commands = []cli.Command{
		initCommand, // 初始化命令
		runCommand,  // 运行命令
	}

	// 在应用执行前进行一些设置
	app.Before = func(context *cli.Context) error {
		// 设置日志格式为 JSON 格式
		log.SetFormatter(&log.JSONFormatter{})
		// 设置日志输出到标准输出
		log.SetOutput(os.Stdout)
		return nil
	}

	// 执行应用命令，如果出错则打印错误信息并退出
	if err := app.Run(os.Args); err != nil {
		// 使用 logrus 打印错误信息并退出程序
		log.Fatal(err)
	}
}

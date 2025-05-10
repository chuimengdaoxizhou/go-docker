package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"go-docker/cgroups/subsystems"
	"go-docker/container"
	"go-docker/network"
	"os"
)

// 定义 runCommand 命令：创建一个新的容器，带有命名空间和 cgroups 限制
var runCommand = cli.Command{
	Name:  "run",                                                                                        // 命令名称
	Usage: `Create a container with namespace and cgroups limit ie: mydocker run -ti [image] [command]`, // 命令用法说明
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "ti", // 启用 TTY
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d", // 启动容器并后台运行
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "m", // 设置内存限制
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare", // 设置 CPU 分配比例
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset", // 设置 CPU 核心限制
			Usage: "cpuset limit",
		},
		cli.StringFlag{
			Name:  "name", // 设置容器名称
			Usage: "container name",
		},
		cli.StringFlag{
			Name:  "v", // 设置容器挂载的卷
			Usage: "volume",
		},
		cli.StringSliceFlag{
			Name:  "e", // 设置环境变量
			Usage: "set environment",
		},
		cli.StringFlag{
			Name:  "net", // 设置容器网络
			Usage: "container network",
		},
		cli.StringSliceFlag{
			Name:  "p", // 设置端口映射
			Usage: "port mapping",
		},
	},
	// 处理命令的执行逻辑
	Action: func(context *cli.Context) error {
		// 检查是否传递了容器命令
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container command")
		}

		// 将命令行参数中的容器命令保存到数组中
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}

		// 获取镜像名称（命令行第一个参数）
		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]

		// 检查是否启用了 TTY 和 detach 两个参数，不能同时使用
		createTty := context.Bool("ti")
		detach := context.Bool("d")

		if createTty && detach {
			return fmt.Errorf("ti and d paramter can not both provided")
		}

		// 获取资源限制的配置
		resConf := subsystems.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuSet:      context.String("cpuset"),
			CpuShare:    context.String("cpushare"),
		}

		log.Infof("createTty %v", createTty)

		// 获取容器名称、卷、网络、环境变量和端口映射
		containerName := context.String("name")
		volume := context.String("v")
		network := context.String("net")
		envSlice := context.StringSlice("e")
		portmapping := context.StringSlice("p")

		// 调用 Run 函数启动容器
		Run(createTty, cmdArray, &resConf, containerName, volume, imageName, envSlice, network, portmapping)
		return nil
	},
}

// 定义 initCommand 命令：初始化容器进程并运行用户的进程
var initCommand = cli.Command{
	Name:  "init",                                                                           // 命令名称
	Usage: "Init container process run user's process in container. Do not call it outside", // 命令用法说明
	Action: func(context *cli.Context) error {
		log.Infof("init come on")
		// 调用容器初始化进程的函数
		err := container.RunContainerInitProcess()
		return err
	},
}

// 定义 listCommand 命令：列出所有容器
var listCommand = cli.Command{
	Name:  "ps",                      // 命令名称
	Usage: "list all the containers", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 调用 ListContainers 函数列出容器
		ListContainers()
		return nil
	},
}

// 定义 logCommand 命令：打印指定容器的日志
var logCommand = cli.Command{
	Name:  "logs",                      // 命令名称
	Usage: "print logs of a container", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 检查是否提供了容器名称
		if len(context.Args()) < 1 {
			return fmt.Errorf("Please input your container name")
		}
		containerName := context.Args().Get(0)
		// 调用 logContainer 函数打印容器日志
		logContainer(containerName)
		return nil
	},
}

// 定义 execCommand 命令：在容器中执行命令
var execCommand = cli.Command{
	Name:  "exec",                          // 命令名称
	Usage: "exec a command into container", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 如果是回调操作，返回
		if os.Getenv(ENV_EXEC_PID) != "" {
			log.Infof("pid callback pid %s", os.Getgid())
			return nil
		}

		// 检查是否提供了容器名称和命令
		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing container name or command")
		}
		containerName := context.Args().Get(0)
		// 获取容器内要执行的命令
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		// 调用 ExecContainer 函数在容器中执行命令
		ExecContainer(containerName, commandArray)
		return nil
	},
}

// 定义 stopCommand 命令：停止容器
var stopCommand = cli.Command{
	Name:  "stop",             // 命令名称
	Usage: "stop a container", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 检查是否提供了容器名称
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		// 调用 stopContainer 函数停止容器
		stopContainer(containerName)
		return nil
	},
}

// 定义 removeCommand 命令：删除容器
var removeCommand = cli.Command{
	Name:  "rm",                       // 命令名称
	Usage: "remove unused containers", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 检查是否提供了容器名称
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		// 调用 removeContainer 函数删除容器
		removeContainer(containerName)
		return nil
	},
}

// 定义 commitCommand 命令：将容器提交为镜像
var commitCommand = cli.Command{
	Name:  "commit",                        // 命令名称
	Usage: "commit a container into image", // 命令用法说明
	Action: func(context *cli.Context) error {
		// 检查是否提供了容器名称和镜像名称
		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing container name and image name")
		}
		containerName := context.Args().Get(0)
		imageName := context.Args().Get(1)
		// 调用 commitContainer 函数将容器提交为镜像
		commitContainer(containerName, imageName)
		return nil
	},
}

// 定义 networkCommand 命令：容器网络命令
var networkCommand = cli.Command{
	Name:  "network",                    // 命令名称
	Usage: "container network commands", // 命令用法说明
	Subcommands: []cli.Command{
		{
			Name:  "create",                     // 创建网络命令
			Usage: "create a container network", // 命令用法说明
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver", // 网络驱动
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet", // 子网 CIDR
					Usage: "subnet cidr",
				},
			},
			Action: func(context *cli.Context) error {
				// 检查是否提供了网络名称
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				// 初始化网络并创建网络
				network.Init()
				err := network.CreateNetwork(context.String("driver"), context.String("subnet"), context.Args()[0])
				if err != nil {
					return fmt.Errorf("create network error: %+v", err)
				}
				return nil
			},
		},
		{
			Name:  "list",                   // 列出网络命令
			Usage: "list container network", // 命令用法说明
			Action: func(context *cli.Context) error {
				// 初始化网络并列出网络
				network.Init()
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",                   // 删除网络命令
			Usage: "remove container network", // 命令用法说明
			Action: func(context *cli.Context) error {
				// 检查是否提供了网络名称
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				// 初始化网络并删除网络
				network.Init()
				err := network.DeleteNetwork(context.Args()[0])
				if err != nil {
					return fmt.Errorf("remove network error: %+v", err)
				}
				return nil
			},
		},
	},
}

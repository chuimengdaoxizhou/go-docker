package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

// ------------------------
// 常量与路径配置
// ------------------------

var (
	RUNNING             string = "running"               // 容器运行状态
	STOP                string = "stopped"               // 容器停止状态
	Exit                string = "exited"                // 容器退出状态
	DefaultInfoLocation string = "/var/run/mydocker/%s/" // 容器信息存储目录（如 config.json）
	ConfigName          string = "config.json"           // 容器配置信息文件名
	ContainerLogFile    string = "container.log"         // 容器标准输出日志文件名
	RootUrl             string = "/root"                 // 镜像只读层路径
	MntUrl              string = "/root/mnt/%s"          // 容器挂载点路径
	WriteLayerUrl       string = "/root/writeLayer/%s"   // 容器可写层路径
)

// ------------------------
// 容器元信息结构体
// ------------------------

type ContainerInfo struct {
	Pid         string   `json:"pid"`         // 容器中 init 进程的 PID（宿主机上的）
	Id          string   `json:"id"`          // 容器 ID
	Name        string   `json:"name"`        // 容器名称
	Command     string   `json:"command"`     // 容器启动时执行的命令
	CreatedTime string   `json:"createTime"`  // 容器创建时间
	Status      string   `json:"status"`      // 容器当前状态（running, stopped 等）
	Volume      string   `json:"volume"`      // 数据卷（volume）挂载路径
	PortMapping []string `json:"portmapping"` // 容器和宿主机端口映射信息
}

// ------------------------
// 创建新的父进程（容器 init 进程）
// tty 表示是否开启终端（即是否交互）
// containerName 是容器名
// volume 是数据卷挂载信息
// imageName 是镜像名称
// envSlice 是环境变量数组
// 返回创建的命令（即 init 容器进程）和管道写入端
// ------------------------

func NewParentProcess(tty bool, containerName, volume, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
	// 创建匿名管道，用于父子进程间通信
	readPipe, writePipe, err := NewPipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}

	// 获取当前进程执行文件的路径（用于调用自身执行 init 子命令）
	initCmd, err := os.Readlink("/proc/self/exe")
	if err != nil {
		log.Errorf("get init process error %v", err)
		return nil, nil
	}

	// 创建命令行对象，执行自身，并传入 init 子命令（此时执行的是 container/init.go 中的逻辑）
	cmd := exec.Command(initCmd, "init")

	// 设置命名空间隔离标志，类似 docker 的 --net、--pid 等
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | // 主机名
			syscall.CLONE_NEWPID | // PID 进程号
			syscall.CLONE_NEWNS | // 文件系统
			syscall.CLONE_NEWNET | // 网络
			syscall.CLONE_NEWIPC, // IPC 信号量
	}

	if tty {
		// 如果是交互模式，将输入输出错误重定向到当前终端
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 非交互模式，将 stdout 重定向到日志文件
		dirURL := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			log.Errorf("NewParentProcess mkdir %s error %v", dirURL, err)
			return nil, nil
		}
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParentProcess create file %s error %v", stdLogFilePath, err)
			return nil, nil
		}
		cmd.Stdout = stdLogFile
	}

	// 把管道的读端传给子进程（作为 fd 3）
	cmd.ExtraFiles = []*os.File{readPipe}

	// 设置环境变量
	cmd.Env = append(os.Environ(), envSlice...)

	// 设置容器文件系统，包括挂载点
	NewWorkSpace(volume, imageName, containerName)

	// 设置容器进程的工作目录（即挂载后的 mnt 目录）
	cmd.Dir = fmt.Sprintf(MntUrl, containerName)

	// 返回构造好的命令对象和写端管道
	return cmd, writePipe
}

// NewPipe 创建一个匿名管道用于父子进程通信
func NewPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}

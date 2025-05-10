package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/cgroups"
	"go-docker/cgroups/subsystems"
	"go-docker/container"
	"go-docker/network"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Run 函数用于启动一个容器
// tty: 是否启用 TTY（终端交互模式）
// comArray: 容器启动时的命令数组
// res: 容器资源限制配置
// containerName: 容器名称
// volume: 容器挂载的卷
// imageName: 容器镜像名称
// envSlice: 环境变量
// nw: 网络名称
// portmapping: 端口映射配置
func Run(tty bool, comArray []string, res *subsystems.ResourceConfig, containerName, volume, imageName string,
	envSlice []string, nw string, portmapping []string) {
	// 生成一个随机的容器 ID
	containerID := randStringBytes(10)
	// 如果未提供容器名称，使用容器 ID
	if containerName == "" {
		containerName = containerID
	}

	// 创建父进程（容器进程）并获取写管道
	parent, writePipe := container.NewParentProcess(tty, containerName, volume, imageName, envSlice)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}

	// 启动父进程（容器进程）
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	// 记录容器信息
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName, containerID, volume)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	// 使用容器 ID 创建 cgroup 管理器
	cgroupManager := cgroups.NewCgroupManager(containerID)
	defer cgroupManager.Destroy()
	// 设置资源限制并将其应用到容器进程
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	// 如果指定了网络配置，则连接容器到指定的网络
	if nw != "" {
		network.Init()
		containerInfo := &container.ContainerInfo{
			Id:          containerID,
			Pid:         strconv.Itoa(parent.Process.Pid),
			Name:        containerName,
			PortMapping: portmapping,
		}
		// 将容器连接到网络
		if err := network.Connect(nw, containerInfo); err != nil {
			log.Errorf("Error Connect Network %v", err)
			return
		}
	}

	// 发送初始化命令给容器
	sendInitCommand(comArray, writePipe)

	// 如果启用了 TTY 模式，则等待父进程（容器进程）结束
	if tty {
		parent.Wait()
		// 删除容器信息并清理容器的工作空间
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(volume, containerName)
	}

}

// sendInitCommand 函数用于发送容器初始化命令
// comArray: 容器启动时的命令数组
// writePipe: 用于写入容器命令的管道
func sendInitCommand(comArray []string, writePipe *os.File) {
	// 将命令数组拼接为一个命令字符串
	command := strings.Join(comArray, " ")
	log.Infof("command all is %s", command)
	// 将命令写入管道并关闭管道
	writePipe.WriteString(command)
	writePipe.Close()
}

// recordContainerInfo 函数用于记录容器的相关信息
// containerPID: 容器进程的 PID
// commandArray: 容器启动命令
// containerName: 容器名称
// id: 容器 ID
// volume: 容器挂载的卷
func recordContainerInfo(containerPID int, commandArray []string, containerName, id, volume string) (string, error) {
	// 获取当前时间作为容器创建时间
	createTime := time.Now().Format("2006-01-02 15:04:05")
	// 将命令数组拼接为一个命令字符串
	command := strings.Join(commandArray, "")
	// 构建容器信息对象
	containerInfo := &container.ContainerInfo{
		Id:          id,
		Pid:         strconv.Itoa(containerPID),
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Name:        containerName,
		Volume:      volume,
	}

	// 将容器信息对象转为 JSON 字符串
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return "", err
	}
	jsonStr := string(jsonBytes)

	// 创建容器信息保存目录
	dirUrl := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("Mkdir error %s error %v", dirUrl, err)
		return "", err
	}
	// 创建容器信息文件
	fileName := dirUrl + "/" + container.ConfigName
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		log.Errorf("Create file %s error %v", fileName, err)
		return "", err
	}
	// 将容器信息写入文件
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error %v", err)
		return "", err
	}

	// 返回容器名称
	return containerName, nil
}

// deleteContainerInfo 函数用于删除容器的相关信息
// containerId: 容器 ID
func deleteContainerInfo(containerId string) {
	// 获取容器信息目录路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerId)
	// 删除容器信息目录
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove dir %s error %v", dirURL, err)
	}
}

// randStringBytes 函数用于生成指定长度的随机字符串
// n: 字符串的长度
func randStringBytes(n int) string {
	letterBytes := "1234567890"      // 随机字符串的字符集
	rand.Seed(time.Now().UnixNano()) // 设置随机种子
	b := make([]byte, n)             // 创建字节数组
	// 填充字节数组
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	// 返回生成的字符串
	return string(b)
}

package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/container"
	_ "go-docker/nsenter" // 引入 nsenter 包，用于容器内的操作
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const ENV_EXEC_PID = "mydocker_pid" // 环境变量名，用于存储容器的 PID
const ENV_EXEC_CMD = "mydocker_cmd" // 环境变量名，用于存储执行的命令

// ExecContainer 执行指定容器内的命令
func ExecContainer(containerName string, comArray []string) {
	// 获取容器的 PID
	pid, err := GetContainerPidByName(containerName)
	if err != nil {
		log.Errorf("Exec container getContainerPidByName %s error %v", containerName, err)
		return
	}

	// 将命令数组转化为字符串
	cmdStr := strings.Join(comArray, " ")
	log.Infof("container pid %s", pid) // 打印容器的 PID
	log.Infof("command %s", cmdStr)    // 打印即将执行的命令

	// 创建一个新的命令，执行当前程序本身，以便进入容器的命名空间
	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin   // 将标准输入传递给命令
	cmd.Stdout = os.Stdout // 将标准输出传递给命令
	cmd.Stderr = os.Stderr // 将标准错误输出传递给命令

	// 设置环境变量：容器 PID 和执行的命令
	os.Setenv(ENV_EXEC_PID, pid)
	os.Setenv(ENV_EXEC_CMD, cmdStr)

	// 获取容器内的环境变量
	containerEnvs := getEnvsByPid(pid)
	cmd.Env = append(os.Environ(), containerEnvs...) // 将容器的环境变量添加到执行命令的环境中

	// 执行命令
	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerName, err) // 执行失败时输出错误
	}
}

// GetContainerPidByName 根据容器名称获取容器的 PID
func GetContainerPidByName(containerName string) (string, error) {
	// 构造容器的配置信息文件路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFilePath := dirURL + container.ConfigName

	// 读取配置文件
	contentBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return "", err // 如果读取文件出错，返回错误
	}

	// 解析配置文件，获取容器信息
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return "", err // 如果解析出错，返回错误
	}

	// 返回容器的 PID
	return containerInfo.Pid, nil
}

// getEnvsByPid 根据容器的 PID 获取容器的环境变量
func getEnvsByPid(pid string) []string {
	// 构造读取容器环境变量的路径
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Read file %s error %v", path, err) // 如果读取失败，记录错误
		return nil                                     // 返回空数组
	}

	// 按照 \u0000 分割环境变量
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs // 返回环境变量数组
}

package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/container"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
)

// stopContainer 函数用于停止指定名称的容器
// containerName: 容器的名称
func stopContainer(containerName string) {
	// 获取容器的 PID
	pid, err := GetContainerPidByName(containerName)
	if err != nil {
		log.Errorf("Get contaienr pid by name %s error %v", containerName, err)
		return
	}

	// 将 PID 从字符串转换为整数
	pidInt, err := strconv.Atoi(pid)
	if err != nil {
		log.Errorf("Conver pid from string to int error %v", err)
		return
	}

	// 发送 SIGTERM 信号停止容器进程
	if err := syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		log.Errorf("Stop container %s error %v", containerName, err)
		return
	}

	// 获取容器的当前状态信息
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerName, err)
		return
	}

	// 更新容器状态为 STOP，并清空 PID
	containerInfo.Status = container.STOP
	containerInfo.Pid = " "

	// 将更新后的容器信息转换为 JSON 格式
	newContentBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Json marshal %s error %v", containerName, err)
		return
	}

	// 获取容器信息的存储目录路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFilePath := dirURL + container.ConfigName

	// 将更新后的容器信息写回到文件
	if err := ioutil.WriteFile(configFilePath, newContentBytes, 0622); err != nil {
		log.Errorf("Write file %s error", configFilePath, err)
	}
}

// getContainerInfoByName 函数根据容器名称获取容器的信息
// containerName: 容器的名称
func getContainerInfoByName(containerName string) (*container.ContainerInfo, error) {
	// 获取容器信息文件的路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFilePath := dirURL + container.ConfigName

	// 读取容器信息文件内容
	contentBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Errorf("Read file %s error %v", configFilePath, err)
		return nil, err
	}

	// 将读取到的内容解析为容器信息结构体
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(contentBytes, &containerInfo); err != nil {
		log.Errorf("GetContainerInfoByName unmarshal error %v", err)
		return nil, err
	}

	// 返回解析后的容器信息
	return &containerInfo, nil
}

// removeContainer 函数用于删除指定名称的容器
// containerName: 容器的名称
func removeContainer(containerName string) {
	// 获取容器的当前状态信息
	containerInfo, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerName, err)
		return
	}

	// 检查容器是否已停止，只有停止的容器才能删除
	if containerInfo.Status != container.STOP {
		log.Errorf("Couldn't remove running container")
		return
	}

	// 获取容器信息文件的存储目录路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)

	// 删除容器信息文件和相关目录
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove file %s error %v", dirURL, err)
		return
	}

	// 删除容器的工作空间
	container.DeleteWorkSpace(containerInfo.Volume, containerName)
}

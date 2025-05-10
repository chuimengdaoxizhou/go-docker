package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/container"
	"io/ioutil"
	"os"
	"text/tabwriter"
)

// ListContainers 列出所有容器的信息
func ListContainers() {
	// 构造容器信息目录的路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, "")
	dirURL = dirURL[:len(dirURL)-1] // 去掉路径末尾的斜杠

	// 读取该目录下的所有文件
	files, err := ioutil.ReadDir(dirURL)
	if err != nil {
		log.Errorf("Read dir %s error %v", dirURL, err) // 如果读取目录出错，打印错误信息
		return
	}

	var containers []*container.ContainerInfo
	// 遍历目录中的所有文件
	for _, file := range files {
		// 如果文件名为 "network"，则跳过该文件（这是一个特殊文件，不是容器的配置文件）
		if file.Name() == "network" {
			continue
		}
		// 获取容器的配置信息
		tmpContainer, err := getContainerInfo(file)
		if err != nil {
			log.Errorf("Get container info error %v", err) // 如果获取容器信息出错，打印错误信息并继续
			continue
		}
		// 将容器信息添加到列表中
		containers = append(containers, tmpContainer)
	}

	// 创建一个 tabwriter 用于格式化输出
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// 输出表头
	fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")

	// 遍历容器列表，打印每个容器的信息
	for _, item := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Id,
			item.Name,
			item.Pid,
			item.Status,
			item.Command,
			item.CreatedTime)
	}

	// 刷新输出缓冲区，确保所有内容都输出
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err) // 如果刷新时出错，打印错误信息
		return
	}
}

// getContainerInfo 获取指定文件对应的容器的配置信息
func getContainerInfo(file os.FileInfo) (*container.ContainerInfo, error) {
	// 获取容器的名称
	containerName := file.Name()

	// 构造容器配置信息文件的路径
	configFileDir := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	configFileDir = configFileDir + container.ConfigName

	// 读取容器配置信息文件
	content, err := ioutil.ReadFile(configFileDir)
	if err != nil {
		log.Errorf("Read file %s error %v", configFileDir, err) // 如果读取文件失败，打印错误信息
		return nil, err
	}

	// 解析容器的配置信息
	var containerInfo container.ContainerInfo
	if err := json.Unmarshal(content, &containerInfo); err != nil {
		log.Errorf("Json unmarshal error %v", err) // 如果 JSON 解析失败，打印错误信息
		return nil, err
	}

	// 返回容器的配置信息
	return &containerInfo, nil
}

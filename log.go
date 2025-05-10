package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/container"
	"io/ioutil"
	"os"
)

// logContainer 打印指定容器的日志信息
func logContainer(containerName string) {
	// 构造容器信息目录路径
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	// 构造容器日志文件的路径
	logFileLocation := dirURL + container.ContainerLogFile

	// 打开容器的日志文件
	file, err := os.Open(logFileLocation)
	// 确保文件打开后关闭
	defer file.Close()

	// 如果打开文件时出错，打印错误信息并返回
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFileLocation, err)
		return
	}

	// 读取文件内容
	content, err := ioutil.ReadAll(file)
	// 如果读取文件时出错，打印错误信息并返回
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFileLocation, err)
		return
	}

	// 打印容器日志内容到标准输出
	fmt.Fprint(os.Stdout, string(content))
}

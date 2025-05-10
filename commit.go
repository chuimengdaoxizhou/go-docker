package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"go-docker/container"
	"os/exec"
)

// commitContainer 将指定容器的文件系统打包并保存为镜像文件
func commitContainer(containerName, imageName string) {
	// 构造容器挂载路径
	mntURL := fmt.Sprintf(container.MntUrl, containerName) // 获取容器的挂载路径模板并格式化为指定容器名的路径
	mntURL += "/"                                          // 加上斜杠以确保路径格式正确

	// 构造要保存的镜像文件路径
	imageTar := container.RootUrl + "/" + imageName + ".tar" // 组合镜像存储的完整路径

	// 使用 tar 命令将容器的文件系统打包为 tar 压缩包
	// 其中 -C 选项用于改变工作目录，表示打包 mntURL 目录下的所有内容
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntURL, ".").CombinedOutput(); err != nil {
		// 如果打包过程出错，记录错误信息
		log.Errorf("Tar folder %s error %v", mntURL, err)
	}
}

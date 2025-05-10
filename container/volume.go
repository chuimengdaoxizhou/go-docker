package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

// 创建 AUFS 文件系统作为容器的工作目录
func NewWorkSpace(volume, imageName, containerName string) {
	// 创建只读层（镜像解压缩）
	CreateReadOnlyLayer(imageName)
	// 创建可写层（容器独立写操作）
	CreateWriteLayer(containerName)
	// 将只读层和可写层挂载到一起，形成统一视图
	CreateMountPoint(containerName, imageName)

	// 如果指定了 volume 参数（数据卷挂载）
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		// 确保 volume 参数格式正确，例如 /宿主机路径:/容器路径
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(volumeURLs, containerName)
			log.Infof("NewWorkSpace volume urls %q", volumeURLs)
		} else {
			log.Infof("Volume parameter input is not correct.")
		}
	}
}

// 解压镜像文件，构建只读层（RO Layer）
func CreateReadOnlyLayer(imageName string) error {
	unTarFolderUrl := RootUrl + "/" + imageName + "/" // 解压目录
	imageUrl := RootUrl + "/" + imageName + ".tar"    // 镜像 tar 包路径

	// 检查是否已解压过该镜像
	exist, err := PathExists(unTarFolderUrl)
	if err != nil {
		log.Infof("Fail to judge whether dir %s exists. %v", unTarFolderUrl, err)
		return err
	}

	if !exist {
		// 创建解压目录
		if err := os.MkdirAll(unTarFolderUrl, 0622); err != nil {
			log.Errorf("Mkdir %s error %v", unTarFolderUrl, err)
			return err
		}

		// 解压 tar 包到只读目录
		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", unTarFolderUrl).CombinedOutput(); err != nil {
			log.Errorf("Untar dir %s error %v", unTarFolderUrl, err)
			return err
		}
	}
	return nil
}

// 创建可写层（RW Layer），用来存放容器写操作的数据
func CreateWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.MkdirAll(writeURL, 0777); err != nil {
		log.Infof("Mkdir write layer dir %s error. %v", writeURL, err)
	}
}

// 挂载数据卷，将宿主机目录挂载到容器目录
func MountVolume(volumeURLs []string, containerName string) error {
	parentUrl := volumeURLs[0] // 宿主机目录
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		log.Infof("Mkdir parent dir %s error. %v", parentUrl, err)
	}
	containerUrl := volumeURLs[1] // 容器内部目录
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerVolumeURL := mntURL + "/" + containerUrl // 容器挂载路径
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Infof("Mkdir container dir %s error. %v", containerVolumeURL, err)
	}
	dirs := "dirs=" + parentUrl
	// 使用 AUFS 把宿主机目录挂载到容器路径上
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL).CombinedOutput()
	if err != nil {
		log.Errorf("Mount volume failed. %v", err)
		return err
	}
	return nil
}

// 把只读层和写层合并挂载到容器的挂载点上
func CreateMountPoint(containerName, imageName string) error {
	mntUrl := fmt.Sprintf(MntUrl, containerName)
	if err := os.MkdirAll(mntUrl, 0777); err != nil {
		log.Errorf("Mkdir mountpoint dir %s error. %v", mntUrl, err)
		return err
	}
	tmpWriteLayer := fmt.Sprintf(WriteLayerUrl, containerName)
	tmpImageLocation := RootUrl + "/" + imageName
	dirs := "dirs=" + tmpWriteLayer + ":" + tmpImageLocation // 注意顺序，写层在上，只读层在下
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntUrl).CombinedOutput()
	if err != nil {
		log.Errorf("Run command for creating mount point failed %v", err)
		return err
	}
	return nil
}

// 容器退出时，清理挂载目录和可写层
func DeleteWorkSpace(volume, containerName string) {
	// 卸载数据卷
	if volume != "" {
		volumeURLs := strings.Split(volume, ":")
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteVolume(volumeURLs, containerName)
		}
	}
	DeleteMountPoint(containerName)
	DeleteWriteLayer(containerName)
}

// 卸载容器 AUFS 文件系统挂载点，并删除挂载目录
func DeleteMountPoint(containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	_, err := exec.Command("umount", mntURL).CombinedOutput()
	if err != nil {
		log.Errorf("Unmount %s error %v", mntURL, err)
		return err
	}
	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove mountpoint dir %s error %v", mntURL, err)
		return err
	}
	return nil
}

// 卸载挂载的 volume 卷
func DeleteVolume(volumeURLs []string, containerName string) error {
	mntURL := fmt.Sprintf(MntUrl, containerName)
	containerUrl := mntURL + "/" + volumeURLs[1]
	if _, err := exec.Command("umount", containerUrl).CombinedOutput(); err != nil {
		log.Errorf("Umount volume %s failed. %v", containerUrl, err)
		return err
	}
	return nil
}

// 删除写层目录
func DeleteWriteLayer(containerName string) {
	writeURL := fmt.Sprintf(WriteLayerUrl, containerName)
	if err := os.RemoveAll(writeURL); err != nil {
		log.Infof("Remove writeLayer dir %s error %v", writeURL, err)
	}
}

// 判断文件或目录是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

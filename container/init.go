package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// RunContainerInitProcess 是容器内部 init 进程的入口函数。
// 它负责从管道中读取用户命令，设置挂载点，并使用 syscall.Exec 执行用户指定的命令。
func RunContainerInitProcess() error {
	cmdArray := readUserCommand()
	if cmdArray == nil || len(cmdArray) == 0 {
		return fmt.Errorf("Run container get user command error, cmdArray is nil")
	}

	// 设置挂载点：proc 文件系统等（容器隔离环境的关键）
	setUpMount()

	// 查找命令的绝对路径
	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find path %s", path)

	// 执行用户命令，替换当前 init 进程（不返回）
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

// 从 3 号文件描述符（即管道）读取用户传入的命令字符串，并解析为字符串数组
func readUserCommand() []string {
	pipe := os.NewFile(uintptr(3), "pipe") // 对应父进程传入的 writePipe
	defer pipe.Close()
	msg, err := ioutil.ReadAll(pipe) // 读取全部内容
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ") // 按空格切分成命令+参数数组
}

/*
 * 设置容器的挂载点（类似 chroot 的环境隔离）
 * - 设置新的 root 文件系统
 * - 挂载 /proc 和 /dev 等必要文件系统
 */
func setUpMount() {
	pwd, err := os.Getwd() // 获取当前工作目录，作为新的 root
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)
	pivotRoot(pwd) // 执行 root 切换操作

	// 挂载 proc 文件系统，便于容器内 ps 等命令访问进程信息
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// 挂载 tmpfs 到 /dev，提供一些必要的设备支持（如 /dev/null）
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

/*
 * 实现 pivot_root 的逻辑，用于将当前容器的 root 文件系统切换为用户指定的目录。
 * pivot_root 会把旧 root 临时移动到 .pivot_root 目录中，并以新 root 为主环境。
 */
func pivotRoot(root string) error {
	// 1. 把自己 bind mount 一次（让新旧 root 不在同一挂载点下）
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	}

	// 2. 创建 .pivot_root 临时目录，用来保存旧的 root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}

	// 3. 执行 pivot_root，切换 root 文件系统
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}

	// 4. 切换工作目录到新的 /
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}

	// 5. 卸载旧 root 挂载点
	pivotDir = filepath.Join("/", ".pivot_root")
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	}

	// 6. 删除旧 root 临时目录
	return os.Remove(pivotDir)
}

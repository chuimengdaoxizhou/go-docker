package subsystems

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

// FindCgroupMountpoint 查找某个子系统（如 "cpu", "memory", "cpuset"）在 cgroup 中的挂载点
// 挂载点信息位于 /proc/self/mountinfo 文件中
// 返回值为该子系统对应的挂载路径，例如 "/sys/fs/cgroup/memory"
func FindCgroupMountpoint(subsystem string) string {
	// 打开 /proc/self/mountinfo 文件读取当前进程的挂载信息
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	// 按行扫描文件内容
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		txt := scanner.Text()
		fields := strings.Split(txt, " ")
		// 每行最后一列字段包含了挂载的子系统名（以逗号分隔）
		for _, opt := range strings.Split(fields[len(fields)-1], ",") {
			if opt == subsystem {
				// 第5列是挂载点路径（即该 subsystem 的根目录）
				return fields[4]
			}
		}
	}
	// 如果扫描过程中出错，则返回空字符串
	if err := scanner.Err(); err != nil {
		return ""
	}

	// 未找到匹配的 subsystem，返回空字符串
	return ""
}

// GetCgroupPath 获取某个子系统下的指定 cgroup 路径
// subsystem：子系统名（如 "cpu", "memory"）
// cgroupPath：用户传入的 cgroup 相对路径（如 "test/mygroup"）
// autoCreate：是否自动创建该路径
// 返回值为完整的 cgroup 绝对路径（如 "/sys/fs/cgroup/memory/test/mygroup"）
func GetCgroupPath(subsystem string, cgroupPath string, autoCreate bool) (string, error) {
	// 查找 subsystem 对应的挂载点路径
	cgroupRoot := FindCgroupMountpoint(subsystem)

	// 拼接成完整路径
	fullPath := path.Join(cgroupRoot, cgroupPath)

	// 判断路径是否存在，若不存在且允许自动创建，则创建目录
	if _, err := os.Stat(fullPath); err == nil || (autoCreate && os.IsNotExist(err)) {
		// 如果不存在，且 autoCreate 为 true，则创建目录
		if os.IsNotExist(err) {
			if err := os.Mkdir(fullPath, 0755); err != nil {
				return "", fmt.Errorf("error create cgroup %v", err)
			}
		}
		// 返回最终的 cgroup 路径
		return fullPath, nil
	} else {
		// 既不存在，也不能创建，或出现其他错误
		return "", fmt.Errorf("cgroup path error %v", err)
	}
}

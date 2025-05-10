package subsystems

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

// MemorySubSystem 表示 memory 子系统，实现了 Subsystem 接口。
// memory 子系统用于限制容器可使用的最大内存字节数。
type MemorySubSystem struct {
}

// Set 设置某个 cgroup 在 memory 子系统中的内存限制
func (s *MemorySubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	// 获取 memory 子系统对应的 cgroup 路径（不存在则创建）
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err == nil {
		// 如果配置了内存限制
		if res.MemoryLimit != "" {
			// 向 memory.limit_in_bytes 文件中写入限制值（单位是字节，字符串类型）
			if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"), []byte(res.MemoryLimit), 0644); err != nil {
				return fmt.Errorf("设置内存限制失败: %v", err)
			}
		}
		return nil
	} else {
		// 获取路径失败，返回错误
		return err
	}
}

// Remove 删除某个 cgroup 在 memory 子系统中的目录
func (s *MemorySubSystem) Remove(cgroupPath string) error {
	// 获取 memory 子系统中的 cgroup 路径（不自动创建）
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 删除该 cgroup 目录及其中的文件
		return os.RemoveAll(subsysCgroupPath)
	} else {
		return err
	}
}

// Apply 将某个进程（pid）加入到该 memory cgroup 中
func (s *MemorySubSystem) Apply(cgroupPath string, pid int) error {
	// 获取 memory 子系统中对应的 cgroup 路径
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 向 tasks 文件中写入进程 pid，表示将该进程加入 cgroup
		if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("将进程加入 memory cgroup 失败: %v", err)
		}
		return nil
	} else {
		return fmt.Errorf("获取 memory cgroup 路径失败 %s: %v", cgroupPath, err)
	}
}

// Name 返回该子系统的名称（用于在 /sys/fs/cgroup 下定位该子系统目录）
func (s *MemorySubSystem) Name() string {
	return "memory"
}

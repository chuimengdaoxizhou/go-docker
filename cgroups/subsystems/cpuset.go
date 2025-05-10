package subsystems

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

// CpusetSubSystem 代表 cpuset 子系统，实现了 Subsystem 接口。
// cpuset 用于指定进程可以在哪些 CPU 上运行（CPU 亲和性）
type CpusetSubSystem struct {
}

// Set 设置某个 cgroup 在 cpuset 子系统中的资源限制（如绑定的 CPU 核心）
func (s *CpusetSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	// 获取对应 subsystem 的 cgroup 路径，如果不存在则创建
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err == nil {
		// 如果配置了 CPU 核（如 "0", "0-2", "1,3"）
		if res.CpuSet != "" {
			// 写入 cpuset.cpus 文件，指定该 cgroup 可使用哪些 CPU 核
			if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "cpuset.cpus"), []byte(res.CpuSet), 0644); err != nil {
				return fmt.Errorf("设置 cpuset 失败: %v", err)
			}
		}
		return nil
	} else {
		// 获取 cgroup 路径失败
		return err
	}
}

// Remove 删除某个 cgroup 在 cpuset 子系统中的目录
func (s *CpusetSubSystem) Remove(cgroupPath string) error {
	// 获取 cgroup 对应的 cpuset 子系统路径，不创建
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 删除整个 cgroup 目录（释放资源）
		return os.RemoveAll(subsysCgroupPath)
	} else {
		// 获取路径失败
		return err
	}
}

// Apply 将某个进程加入到该 cpuset cgroup 中
func (s *CpusetSubSystem) Apply(cgroupPath string, pid int) error {
	// 获取该 cgroup 在 cpuset 子系统中的路径
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 将进程 pid 写入 tasks 文件，让该进程属于此 cgroup
		if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("将进程加入 cpuset cgroup 失败: %v", err)
		}
		return nil
	} else {
		// 获取路径失败
		return fmt.Errorf("获取 cgroup 路径 %s 失败: %v", cgroupPath, err)
	}
}

// Name 返回该子系统的名称，用于作为 /sys/fs/cgroup 下对应目录名
func (s *CpusetSubSystem) Name() string {
	return "cpuset"
}

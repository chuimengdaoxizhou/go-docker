package subsystems

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

// CpuSubSystem 代表 CPU 子系统，实现了 Subsystem 接口
type CpuSubSystem struct {
}

// Set 设置某个 cgroup 在 CPU 子系统中的资源限制（如 cpu.shares）
func (s *CpuSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	// 获取对应 subsystem 的 cgroup 路径，如果不存在则创建
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, true); err == nil {
		// 如果用户配置了 CPU 共享权重
		if res.CpuShare != "" {
			// 将 CPU 权重写入 cpu.shares 文件
			if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "cpu.shares"), []byte(res.CpuShare), 0644); err != nil {
				return fmt.Errorf("设置 cgroup 的 cpu.shares 失败: %v", err)
			}
		}
		return nil
	} else {
		// 获取 cgroup 路径失败
		return err
	}
}

// Remove 删除某个 cgroup 在 CPU 子系统中的目录
func (s *CpuSubSystem) Remove(cgroupPath string) error {
	// 获取对应 subsystem 的 cgroup 路径，不创建
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 删除该路径及其所有内容
		return os.RemoveAll(subsysCgroupPath)
	} else {
		// 获取路径失败
		return err
	}
}

// Apply 将某个进程（pid）加入到对应的 cgroup 中
func (s *CpuSubSystem) Apply(cgroupPath string, pid int) error {
	// 获取对应 subsystem 的 cgroup 路径
	if subsysCgroupPath, err := GetCgroupPath(s.Name(), cgroupPath, false); err == nil {
		// 将进程 pid 写入 tasks 文件，表示该进程属于该 cgroup
		if err := ioutil.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), 0644); err != nil {
			return fmt.Errorf("将进程加入 cgroup 失败: %v", err)
		}
		return nil
	} else {
		// 获取路径失败
		return fmt.Errorf("获取 cgroup 路径 %s 失败: %v", cgroupPath, err)
	}
}

// Name 返回 CPU 子系统的名称，即 "cpu"
// 用于与 /sys/fs/cgroup 下的目录名对应
func (s *CpuSubSystem) Name() string {
	return "cpu"
}

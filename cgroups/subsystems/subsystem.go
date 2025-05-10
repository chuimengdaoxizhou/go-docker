package subsystems

// ResourceConfig 用于传递资源限制配置参数
// 可以通过该结构体配置内存限制、CPU 时间片权重、以及绑定的 CPU 核心
type ResourceConfig struct {
	// MemoryLimit 表示容器可使用的最大内存（单位：字节，字符串形式，如 "500m"）
	MemoryLimit string

	// CpuShare 表示 CPU 使用的权重（相对值），值越大分配的 CPU 时间片越多（如 "1024"）
	CpuShare string

	// CpuSet 表示允许容器运行在哪些 CPU 核心上（如 "0-2", "0,1"）
	CpuSet string
}

// Subsystem 是所有 cgroup 子系统的统一接口
// 每个资源子系统（如 memory/cpu/cpuset）都需要实现该接口，便于统一操作
type Subsystem interface {
	// Name 返回子系统的名字（对应 cgroup 中的子目录名，如 "memory", "cpu"）
	Name() string

	// Set 设置某个 cgroup 在该子系统中的资源限制
	Set(path string, res *ResourceConfig) error

	// Apply 将进程加入该子系统对应的 cgroup 中
	Apply(path string, pid int) error

	// Remove 移除该子系统中指定的 cgroup
	Remove(path string) error
}

// SubsystemsIns 是当前系统中支持的所有 Subsystem 实例的集合
// 这些子系统会被统一调用，执行资源限制设置、进程绑定和清理操作
var (
	SubsystemsIns = []Subsystem{
		&CpusetSubSystem{}, // CPU 核绑定子系统
		&MemorySubSystem{}, // 内存限制子系统
		&CpuSubSystem{},    // CPU 时间片控制子系统
	}
)

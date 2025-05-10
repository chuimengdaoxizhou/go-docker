package network

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go-docker/container"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
)

var (
	defaultNetworkPath = "/var/run/mydocker/network/network/" // 默认的网络存储路径
	drivers            = map[string]NetworkDriver{}           // 网络驱动映射
	networks           = map[string]*Network{}                // 网络映射
)

// 网络端点结构体，表示容器的网络接口
type Endpoint struct {
	ID          string           `json:"id"`  // 端点ID
	Device      netlink.Veth     `json:"dev"` // 虚拟网卡设备
	IPAddress   net.IP           `json:"ip"`  // IP地址
	MacAddress  net.HardwareAddr `json:"mac"` // MAC地址
	Network     *Network         // 所在的网络
	PortMapping []string         // 端口映射
}

// 网络结构体，表示一个网络
type Network struct {
	Name    string     // 网络名称
	IpRange *net.IPNet // IP范围
	Driver  string     // 网络驱动
}

// 网络驱动接口，提供网络的基本操作方法
type NetworkDriver interface {
	Name() string                                         // 返回驱动名称
	Create(subnet string, name string) (*Network, error)  // 创建网络
	Delete(network Network) error                         // 删除网络
	Connect(network *Network, endpoint *Endpoint) error   // 连接网络
	Disconnect(network Network, endpoint *Endpoint) error // 断开网络
}

// 将网络信息保存到指定的路径
func (nw *Network) dump(dumpPath string) error {
	// 判断存储路径是否存在，不存在则创建
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
		} else {
			return err
		}
	}

	// 生成网络文件路径
	nwPath := path.Join(dumpPath, nw.Name)
	// 打开文件（以写入模式）
	nwFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}
	defer nwFile.Close()

	// 将网络信息转为JSON格式
	nwJson, err := json.Marshal(nw)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}

	// 写入文件
	_, err = nwFile.Write(nwJson)
	if err != nil {
		logrus.Errorf("error：", err)
		return err
	}
	return nil
}

// 从指定路径删除网络信息
func (nw *Network) remove(dumpPath string) error {
	// 检查文件是否存在，存在则删除
	if _, err := os.Stat(path.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		return os.Remove(path.Join(dumpPath, nw.Name))
	}
}

// 从指定路径加载网络信息
func (nw *Network) load(dumpPath string) error {
	nwConfigFile, err := os.Open(dumpPath)
	defer nwConfigFile.Close()
	if err != nil {
		return err
	}
	nwJson := make([]byte, 2000)
	n, err := nwConfigFile.Read(nwJson)
	if err != nil {
		return err
	}

	// 将JSON内容解析为网络结构体
	err = json.Unmarshal(nwJson[:n], nw)
	if err != nil {
		logrus.Errorf("Error load nw info", err)
		return err
	}
	return nil
}

// 初始化网络驱动和网络存储路径
func Init() error {
	var bridgeDriver = BridgeNetworkDriver{} // 桥接网络驱动
	drivers[bridgeDriver.Name()] = &bridgeDriver

	// 如果默认的网络路径不存在，则创建
	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
		} else {
			return err
		}
	}

	// 遍历网络存储路径，加载已经存在的网络
	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		if strings.HasSuffix(nwPath, "/") {
			return nil
		}
		_, nwName := path.Split(nwPath)
		nw := &Network{
			Name: nwName,
		}

		// 加载网络信息
		if err := nw.load(nwPath); err != nil {
			logrus.Errorf("error load network: %s", err)
		}

		// 将网络添加到网络映射中
		networks[nwName] = nw
		return nil
	})

	return nil
}

// 创建一个新的网络
func CreateNetwork(driver, subnet, name string) error {
	_, cidr, _ := net.ParseCIDR(subnet)
	// 从IP分配器中分配一个IP
	ip, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = ip

	// 使用指定的驱动创建网络
	nw, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}

	// 保存网络信息
	return nw.dump(defaultNetworkPath)
}

// 列出当前所有网络
func ListNetwork() {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// 打印表头
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")
	for _, nw := range networks {
		// 打印每个网络的信息
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			nw.Name,
			nw.IpRange.String(),
			nw.Driver,
		)
	}
	// 刷新输出
	if err := w.Flush(); err != nil {
		logrus.Errorf("Flush error %v", err)
		return
	}
}

// 删除指定的网络
func DeleteNetwork(networkName string) error {
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	// 释放网络的IP
	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("Error Remove Network gateway ip: %s", err)
	}

	// 删除网络驱动
	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("Error Remove Network DriverError: %s", err)
	}

	// 删除网络文件
	return nw.remove(defaultNetworkPath)
}

// 进入容器的网络命名空间
func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		logrus.Errorf("error get container net namespace, %v", err)
	}

	nsFD := f.Fd()
	runtime.LockOSThread()

	// 将veth的另一端设置到容器的命名空间中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		logrus.Errorf("error set link netns , %v", err)
	}

	// 获取当前的网络命名空间
	origns, err := netns.Get()
	if err != nil {
		logrus.Errorf("error get current netns, %v", err)
	}

	// 设置当前进程的网络命名空间，并在执行完毕后恢复
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		logrus.Errorf("error set netns, %v", err)
	}
	return func() {
		netns.Set(origns)
		origns.Close()
		runtime.UnlockOSThread()
		f.Close()
	}
}

// 配置网络端点的IP地址和路由
func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	// 获取虚拟网卡的设备信息
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	defer enterContainerNetns(&peerLink, cinfo)()

	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress

	// 设置网络接口的IP地址
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}

	// 启动网络接口
	if err = setInterfaceUP(ep.Device.PeerName); err != nil {
		return err
	}

	// 启动回环接口
	if err = setInterfaceUP("lo"); err != nil {
		return err
	}

	// 设置默认路由
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")

	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP,
		Dst:       cidr,
	}

	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}

	return nil
}

// 配置端口映射
func configPortMapping(ep *Endpoint, cinfo *container.ContainerInfo) error {
	for _, pm := range ep.PortMapping {
		// 拆分端口映射
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			logrus.Errorf("port mapping format error, %v", pm)
			continue
		}
		// 构造iptables命令
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		//err := cmd.Run()
		output, err := cmd.Output()
		if err != nil {
			logrus.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return nil
}

// 连接网络到容器
func Connect(networkName string, cinfo *container.ContainerInfo) error {
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	// 分配容器IP地址
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}

	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}
	// 调用网络驱动挂载和配置网络端点
	if err = drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}
	// 到容器的namespace配置容器网络设备IP地址
	if err = configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	return configPortMapping(ep, cinfo)
}

// 断开容器与网络的连接
func Disconnect(networkName string, cinfo *container.ContainerInfo) error {
	return nil
}

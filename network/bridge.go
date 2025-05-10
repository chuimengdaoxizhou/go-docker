package network

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"net"
	"os/exec"
	"strings"
	"time"
)

// BridgeNetworkDriver 是桥接网络的驱动实现
type BridgeNetworkDriver struct {
}

// 返回驱动名称
func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

// 创建桥接网络，subnet 表示子网段，name 是网络名称
func (d *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip // 设置 IP 段的起始地址
	n := &Network{
		Name:    name,
		IpRange: ipRange,
		Driver:  d.Name(),
	}

	// 初始化桥接网络接口
	err := d.initBridge(n)
	if err != nil {
		log.Errorf("error init bridge: %v", err)
	}

	return n, err
}

// 删除网络，对应的是删除 Linux 中的桥接网卡
func (d *BridgeNetworkDriver) Delete(network Network) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	return netlink.LinkDel(br)
}

// 将容器端点连接到网络，主要是创建 veth 设备对，并将一端连接到桥上
func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	// 创建 veth 设备属性，名字用 endpoint ID 的前 5 位
	la := netlink.NewLinkAttrs()
	la.Name = endpoint.ID[:5]
	la.MasterIndex = br.Attrs().Index // 设置主接口为桥接设备

	// 创建一对 veth 接口（主机端、容器端）
	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5], // 容器端接口名
	}

	// 添加 veth 接口到主机网络栈
	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}

	// 启用主机端的 veth 接口
	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("Error Add Endpoint Device: %v", err)
	}
	return nil
}

// 断开容器与网络连接，当前未实现
func (d *BridgeNetworkDriver) Disconnect(network Network, endpoint *Endpoint) error {
	return nil
}

// 初始化桥接设备，包括：创建 bridge、分配 IP、设置 UP、配置 iptables
func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	bridgeName := n.Name

	// 创建桥接接口（如果已存在则跳过）
	if err := createBridgeInterface(bridgeName); err != nil {
		return fmt.Errorf("Error add bridge： %s, Error: %v", bridgeName, err)
	}

	// 设置桥接网卡的网关 IP 地址
	gatewayIP := *n.IpRange
	gatewayIP.IP = n.IpRange.IP

	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return fmt.Errorf("Error assigning address: %s on bridge: %s with an error of: %v", gatewayIP, bridgeName, err)
	}

	// 启用桥接网卡接口
	if err := setInterfaceUP(bridgeName); err != nil {
		return fmt.Errorf("Error set bridge up: %s, Error: %v", bridgeName, err)
	}

	// 配置 iptables，实现容器出站 SNAT（源地址伪装）
	if err := setupIPTables(bridgeName, n.IpRange); err != nil {
		return fmt.Errorf("Error setting iptables for %s: %v", bridgeName, err)
	}

	return nil
}

// 删除桥接网络接口
func (d *BridgeNetworkDriver) deleteBridge(n *Network) error {
	bridgeName := n.Name

	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("Getting link with name %s failed: %v", bridgeName, err)
	}

	if err := netlink.LinkDel(l); err != nil {
		return fmt.Errorf("Failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	return nil
}

// 创建桥接网卡（netlink 实现），如果存在则跳过
func createBridgeInterface(bridgeName string) error {
	_, err := net.InterfaceByName(bridgeName)
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		// 如果网卡存在或错误不是 "找不到接口"，则直接返回
		return err
	}

	// 初始化 netlink 桥接对象
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	br := &netlink.Bridge{LinkAttrs: la}

	// 添加网桥到 Linux 网络栈
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("Bridge creation failed for bridge %s: %v", bridgeName, err)
	}
	return nil
}

// 启动网卡接口
func setInterfaceUP(interfaceName string) error {
	iface, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("Error retrieving a link named [ %s ]: %v", interfaceName, err)
	}

	// 设置为 UP 状态
	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("Error enabling interface for %s: %v", interfaceName, err)
	}
	return nil
}

// 设置网卡 IP 地址
func setInterfaceIP(name string, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error
	for i := 0; i < retries; i++ {
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("获取网卡失败，可能不存在。运行 [ ip link ] 查看详情: %v", err)
	}
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(iface, addr)
}

// 设置 iptables NAT 规则，实现容器访问外网时的源地址转换
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	// -t nat：作用在 NAT 表；-A POSTROUTING：出网前处理；-s：源 IP 网段；
	// ! -o bridgeName：不是从网桥设备出去；-j MASQUERADE：进行源地址伪装
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)

	// 执行 iptables 命令
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}
	return err
}

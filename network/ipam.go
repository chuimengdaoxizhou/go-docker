package network

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"path"
	"strings"
)

const ipamDefaultAllocatorPath = "/var/run/mydocker/network/ipam/subnet.json"

// IPAM（IP Address Management）结构体
type IPAM struct {
	// 保存子网分配信息的持久化文件路径
	SubnetAllocatorPath string
	// 保存每个子网中 IP 地址的分配状态（用二进制字符串 "001010" 表示是否被分配）
	Subnets *map[string]string
}

// 全局 IPAM 对象
var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

// 加载子网分配信息（从文件中反序列化）
func (ipam *IPAM) load() error {
	// 判断文件是否存在
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在就直接返回（说明还未分配过）
		} else {
			return err
		}
	}
	// 打开文件
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}

	// 读取文件内容
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return err
	}

	// JSON 反序列化到 ipam.Subnets 中
	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
		return err
	}
	return nil
}

// 保存子网分配信息到文件中（持久化）
func (ipam *IPAM) dump() error {
	// 获取目录路径
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)

	// 如果目录不存在则创建
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(ipamConfigFileDir, 0644)
		} else {
			return err
		}
	}

	// 以可写方式打开文件，覆盖写入
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}

	// 将子网分配信息序列化为 JSON
	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}

	// 写入文件
	_, err = subnetConfigFile.Write(ipamConfigJson)
	if err != nil {
		return err
	}

	return nil
}

// 从指定子网中分配一个 IP 地址
func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	// 初始化一个空的子网映射
	ipam.Subnets = &map[string]string{}

	// 加载当前已存在的 IP 分配记录
	err = ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	// 重新解析子网（避免指针解析错误）
	_, subnet, _ = net.ParseCIDR(subnet.String())

	// 计算子网掩码能容纳多少主机地址
	one, size := subnet.Mask.Size()

	// 若该子网尚未存在记录，则初始化一个全为 "0" 的字符串，代表每个 IP 均未被分配
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}

	// 遍历子网的 IP 分配记录，找到第一个可用 IP
	for c := range (*ipam.Subnets)[subnet.String()] {
		if (*ipam.Subnets)[subnet.String()][c] == '0' {
			ipalloc := []byte((*ipam.Subnets)[subnet.String()])
			ipalloc[c] = '1' // 标记为已分配
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)

			// 计算分配的 IP 地址
			ip = subnet.IP
			for t := uint(4); t > 0; t -= 1 {
				[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
			}
			ip[3] += 1 // 跳过网关地址
			break
		}
	}

	// 持久化更新后的分配信息
	ipam.dump()
	return
}

// 释放指定子网中分配的 IP 地址
func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	// 初始化子网映射表
	ipam.Subnets = &map[string]string{}

	// 标准化 subnet
	_, subnet, _ = net.ParseCIDR(subnet.String())

	// 加载之前的分配信息
	err := ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	// 计算 IP 地址在分配位图中的索引
	c := 0
	releaseIP := ipaddr.To4()
	releaseIP[3] -= 1 // 还原出原始 IP
	for t := uint(4); t > 0; t -= 1 {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}

	// 将对应位置置为 '0'，表示可用
	ipalloc := []byte((*ipam.Subnets)[subnet.String()])
	ipalloc[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipalloc)

	// 持久化保存
	ipam.dump()
	return nil
}

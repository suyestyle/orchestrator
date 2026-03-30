/*
   Copyright 2015 Shlomi Noach, courtesy Booking.com

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
instance_key.go 定义了 MySQL 实例的唯一标识符和相关操作。

InstanceKey 是 Orchestrator 中最基础的数据类型，用于唯一标识一个 MySQL 实例。
它支持以下功能：

1. 实例标识
   - 通过主机名和端口号唯一标识 MySQL 实例
   - 支持 IPv4 和 IPv6 地址格式
   - 支持主机名解析和反解析

2. 地址解析
   - 主机名到 IP 地址的解析
   - 支持 hostname_resolve 和 hostname_unresolve 映射表
   - 处理各种地址格式的解析

3. 字符串表示
   - 提供多种字符串格式化方法
   - 支持 JSON 序列化和反序列化
   - 便于日志记录和显示

4. 比较操作
   - 支持实例键的相等性比较
   - 支持排序操作

该模块是 Orchestrator 拓扑管理的基础，所有拓扑操作都基于 InstanceKey 进行。
*/
package inst

import (
	"fmt"
	"github.com/openark/orchestrator/go/config"
	"regexp"
	"strconv"
	"strings"
)

// InstanceKey 是实例标识符，通过主机名和端口来唯一标识一个 MySQL 实例
// 这是 Orchestrator 中最基础的数据类型，用于所有拓扑操作
type InstanceKey struct {
	Hostname string // 主机名或 IP 地址
	Port     int    // 端口号，通常是 3306
}

var (
	// IPv4 地址格式的正则表达式，例如：192.168.1.1
	ipv4Regexp = regexp.MustCompile("^([0-9]+)[.]([0-9]+)[.]([0-9]+)[.]([0-9]+)$")

	// IPv4 主机名:端口格式的正则表达式，例如：db.example.com:3306
	ipv4HostPortRegexp = regexp.MustCompile("^([^:]+):([0-9]+)$")

	// 仅主机名格式的正则表达式，例如：db.example.com
	ipv4HostRegexp = regexp.MustCompile("^([^:]+)$")

	// IPv6 主机名:端口格式的正则表达式，例如：[2001:db8:1f70::999:de8:7648:6e8]:3308
	ipv6HostPortRegexp = regexp.MustCompile("^\\[([:0-9a-fA-F]+)\\]:([0-9]+)$")

	// 仅 IPv6 地址格式的正则表达式，例如：2001:db8:1f70::999:de8:7648:6e8
	ipv6HostRegexp = regexp.MustCompile("^([:0-9a-fA-F]+)$")
)

// detachHint 用于标识已分离的实例的特殊前缀
// 当实例从拓扑中分离时，主机名会以此前缀开头
const detachHint = "//"

// newInstanceKey 创建一个新的实例键
// hostname: 主机名或 IP 地址
// port: 端口号
// resolve: 是否执行主机名解析
//
// 返回创建的 InstanceKey 和可能的错误
func newInstanceKey(hostname string, port int, resolve bool) (instanceKey *InstanceKey, err error) {
	if hostname == "" {
		return instanceKey, fmt.Errorf("NewResolveInstanceKey: Empty hostname")
	}

	instanceKey = &InstanceKey{Hostname: hostname, Port: port}
	if resolve {
		instanceKey, err = instanceKey.ResolveHostname()
	}
	return instanceKey, err
}

// newInstanceKeyStrings
func newInstanceKeyStrings(hostname string, port string, resolve bool) (*InstanceKey, error) {
	if portInt, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("Invalid port: %s", port)
	} else {
		return newInstanceKey(hostname, portInt, resolve)
	}
}
func parseRawInstanceKey(hostPort string, resolve bool) (instanceKey *InstanceKey, err error) {
	hostname := ""
	port := ""
	if submatch := ipv4HostPortRegexp.FindStringSubmatch(hostPort); len(submatch) > 0 {
		hostname = submatch[1]
		port = submatch[2]
	} else if submatch := ipv4HostRegexp.FindStringSubmatch(hostPort); len(submatch) > 0 {
		hostname = submatch[1]
	} else if submatch := ipv6HostPortRegexp.FindStringSubmatch(hostPort); len(submatch) > 0 {
		hostname = submatch[1]
		port = submatch[2]
	} else if submatch := ipv6HostRegexp.FindStringSubmatch(hostPort); len(submatch) > 0 {
		hostname = submatch[1]
	} else {
		return nil, fmt.Errorf("Cannot parse address: %s", hostPort)
	}
	if port == "" {
		port = fmt.Sprintf("%d", config.Config.DefaultInstancePort)
	}
	return newInstanceKeyStrings(hostname, port, resolve)
}

func NewResolveInstanceKey(hostname string, port int) (instanceKey *InstanceKey, err error) {
	return newInstanceKey(hostname, port, true)
}

// NewResolveInstanceKeyStrings creates and resolves a new instance key based on string params
func NewResolveInstanceKeyStrings(hostname string, port string) (*InstanceKey, error) {
	return newInstanceKeyStrings(hostname, port, true)
}

func ParseResolveInstanceKey(hostPort string) (instanceKey *InstanceKey, err error) {
	return parseRawInstanceKey(hostPort, true)
}

func ParseRawInstanceKey(hostPort string) (instanceKey *InstanceKey, err error) {
	return parseRawInstanceKey(hostPort, false)
}

// NewResolveInstanceKeyStrings creates and resolves a new instance key based on string params
func NewRawInstanceKeyStrings(hostname string, port string) (*InstanceKey, error) {
	return newInstanceKeyStrings(hostname, port, false)
}

//
func (this *InstanceKey) ResolveHostname() (*InstanceKey, error) {
	if !this.IsValid() {
		return this, nil
	}

	hostname, err := ResolveHostname(this.Hostname)
	if err == nil {
		this.Hostname = hostname
	}
	return this, err
}

// Equals tests equality between this key and another key
func (this *InstanceKey) Equals(other *InstanceKey) bool {
	if other == nil {
		return false
	}
	return this.Hostname == other.Hostname && this.Port == other.Port
}

// SmallerThan returns true if this key is dictionary-smaller than another.
// This is used for consistent sorting/ordering; there's nothing magical about it.
func (this *InstanceKey) SmallerThan(other *InstanceKey) bool {
	if this.Hostname < other.Hostname {
		return true
	}
	if this.Hostname == other.Hostname && this.Port < other.Port {
		return true
	}
	return false
}

// IsDetached returns 'true' when this hostname is logically "detached"
func (this *InstanceKey) IsDetached() bool {
	return strings.HasPrefix(this.Hostname, detachHint)
}

// IsValid uses simple heuristics to see whether this key represents an actual instance
func (this *InstanceKey) IsValid() bool {
	if this.Hostname == "_" {
		return false
	}
	if this.IsDetached() {
		return false
	}
	return len(this.Hostname) > 0 && this.Port > 0
}

// DetachedKey returns an instance key whose hostname is detahced: invalid, but recoverable
func (this *InstanceKey) DetachedKey() *InstanceKey {
	if this.IsDetached() {
		return this
	}
	return &InstanceKey{Hostname: fmt.Sprintf("%s%s", detachHint, this.Hostname), Port: this.Port}
}

// ReattachedKey returns an instance key whose hostname is detahced: invalid, but recoverable
func (this *InstanceKey) ReattachedKey() *InstanceKey {
	if !this.IsDetached() {
		return this
	}
	return &InstanceKey{Hostname: this.Hostname[len(detachHint):], Port: this.Port}
}

// StringCode returns an official string representation of this key
func (this *InstanceKey) StringCode() string {
	return fmt.Sprintf("%s:%d", this.Hostname, this.Port)
}

// DisplayString returns a user-friendly string representation of this key
func (this *InstanceKey) DisplayString() string {
	return this.StringCode()
}

// String returns a user-friendly string representation of this key
func (this InstanceKey) String() string {
	return this.StringCode()
}

// IsValid uses simple heuristics to see whether this key represents an actual instance
func (this *InstanceKey) IsIPv4() bool {
	return ipv4Regexp.MatchString(this.Hostname)
}

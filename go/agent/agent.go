/*
   Copyright 2014 Outbrain Inc.

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
Package agent 提供 Orchestrator Agent 的核心功能。

Orchestrator Agent 是部署在 MySQL 服务器上的轻量级代理程序，提供以下功能：

1. 系统信息收集
   - 服务器硬件信息
   - 磁盘使用情况
   - 网络配置信息
   - 进程状态监控

2. LVM 管理支持
   - LVM 逻辑卷信息收集
   - 快照创建和管理
   - 存储空间监控
   - 卷组状态跟踪

3. MySQL 环境监控
   - MySQL 数据目录监控
   - 数据库进程状态
   - 配置文件检查
   - 日志文件监控

4. 远程操作执行
   - 脚本执行支持
   - 文件传输功能
   - 系统命令执行
   - 状态报告回传

5. 高可用支持
   - 健康状态检查
   - 故障恢复协助
   - 备份恢复操作
   - 拓扑变更支持

Agent 与 Orchestrator 主服务通过 HTTP API 进行通信，提供了 MySQL 环境的
深度集成和自动化管理能力。
*/
package agent

import "github.com/openark/orchestrator/go/inst"

// LogicalVolume 描述一个 LVM 逻辑卷
// 提供完整的 LVM 卷信息，包括快照状态
type LogicalVolume struct {
	Name            string  // 逻辑卷名称
	GroupName       string  // 所属卷组名称
	Path            string  // 逻辑卷设备路径
	IsSnapshot      bool    // 是否为快照卷
	SnapshotPercent float64 // 快照使用百分比（仅快照卷有效）
}

// Mount 描述文件系统挂载点
// 包含磁盘使用情况和 MySQL 相关信息
type Mount struct {
	Path           string // 挂载点路径
	Device         string // 设备名称
	LVPath         string // LVM 逻辑卷路径（如果适用）
	FileSystem     string // 文件系统类型
	IsMounted      bool   // 是否已挂载
	DiskUsage      int64  // 磁盘使用量（字节）
	MySQLDataPath  string // MySQL 数据目录路径
	MySQLDiskUsage int64  // MySQL 数据使用量（字节）
}

// Agent 表示一个 Orchestrator Agent 的完整信息
// 包含系统状态、配置和监控数据
type Agent struct {
	Hostname                string            // Agent 主机名
	Port                    int               // Agent 监听端口
	Token                   string            // 认证令牌
	LastSubmitted           string            // 最后提交时间
	AvailableLocalSnapshots []string          // 可用的本地快照列表
	AvailableSnapshots      []string          // 可用的快照列表
	LogicalVolumes          []LogicalVolume   // LVM 逻辑卷列表
	MountPoint              Mount
	MySQLRunning            bool
	MySQLDiskUsage          int64
	MySQLPort               int64
	MySQLDatadirDiskFree    int64
	MySQLErrorLogTail       []string
}

// SeedOperation makes for the high level data & state of a seed operation
type SeedOperation struct {
	SeedId         int64
	TargetHostname string
	SourceHostname string
	StartTimestamp string
	EndTimestamp   string
	IsComplete     bool
	IsSuccessful   bool
}

// SeedOperationState represents a single state (step) in a seed operation
type SeedOperationState struct {
	SeedStateId    int64
	SeedId         int64
	StateTimestamp string
	Action         string
	ErrorMessage   string
}

// Build an instance key for a given agent
func (this *Agent) GetInstance() *inst.InstanceKey {
	return &inst.InstanceKey{Hostname: this.Hostname, Port: int(this.MySQLPort)}
}

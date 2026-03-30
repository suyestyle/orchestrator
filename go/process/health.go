/*
   Copyright 2017 Shlomi Noach, GitHub Inc.

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
Package process 提供 Orchestrator 的进程管理和健康检查功能。

该包实现了以下核心功能：

1. 进程健康监控
   - 定期健康状态检查
   - 节点存活性监控
   - 健康状态缓存管理
   - 异常状态检测和报告

2. 集群节点管理
   - 多节点协调
   - 节点注册和注销
   - 领导者状态跟踪
   - 节点通信健康

3. 服务可用性管理
   - 服务状态监控
   - 故障检测和报告
   - 自动恢复机制
   - 降级保护

4. 选举和一致性
   - 配合 Raft 模块进行选举
   - 维护集群一致性
   - 处理网络分区
   - 防止脑裂现象

5. 指标收集和报告
   - 健康检查指标
   - 性能统计数据
   - 状态变化事件
   - 告警和通知

该模块是 Orchestrator 高可用架构的重要组成部分，确保集群的
稳定运行和服务的持续可用性。
*/
package process

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/openark/orchestrator/go/config"
	"github.com/openark/orchestrator/go/util"

	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/raft"
	"github.com/patrickmn/go-cache"
)

var (
	// lastHealthCheckUnixNano 记录最后一次健康检查的时间戳（纳秒）
	// 使用原子操作确保并发安全
	lastHealthCheckUnixNano int64

	// lastGoodHealthCheckUnixNano 记录最后一次成功健康检查的时间戳（纳秒）
	// 用于判断服务是否持续健康
	lastGoodHealthCheckUnixNano int64

	// LastContinousCheckHealthy 记录连续健康检查成功的次数
	// 用于判断服务稳定性
	LastContinousCheckHealthy int64

	// lastHealthCheckCache 健康检查结果缓存
	// 避免频繁的健康检查操作，提高系统性能
	lastHealthCheckCache = cache.New(config.HealthPollSeconds*time.Second, time.Second)
)

// NodeHealth 表示集群节点的健康状态信息
// 用于跟踪和管理集群中各个节点的健康状态
type NodeHealth struct {
	Hostname string // 节点主机名
	Token    string // 节点认证令牌
	AppVersion      string
	FirstSeenActive string
	LastSeenActive  string
	ExtraInfo       string
	Command         string
	DBBackend       string

	LastReported time.Time
	onceHistory  sync.Once
	onceUpdate   sync.Once
}

func NewNodeHealth() *NodeHealth {
	return &NodeHealth{
		Hostname:   ThisHostname,
		Token:      util.ProcessToken.Hash,
		AppVersion: config.RuntimeCLIFlags.ConfiguredVersion,
	}
}

func (nodeHealth *NodeHealth) Update() *NodeHealth {
	nodeHealth.onceUpdate.Do(func() {
		nodeHealth.Hostname = ThisHostname
		nodeHealth.Token = util.ProcessToken.Hash
		nodeHealth.AppVersion = config.RuntimeCLIFlags.ConfiguredVersion
	})
	nodeHealth.LastReported = time.Now()
	return nodeHealth
}

var ThisNodeHealth = NewNodeHealth()

type HealthStatus struct {
	Healthy            bool
	Hostname           string
	Token              string
	IsActiveNode       bool
	ActiveNode         NodeHealth
	Error              error
	AvailableNodes     [](*NodeHealth)
	RaftLeader         string
	IsRaftLeader       bool
	RaftLeaderURI      string
	RaftAdvertise      string
	RaftHealthyMembers []string
}

type OrchestratorExecutionMode string

const (
	OrchestratorExecutionCliMode  OrchestratorExecutionMode = "CLIMode"
	OrchestratorExecutionHttpMode                           = "HttpMode"
)

var continuousRegistrationOnce sync.Once

func RegisterNode(nodeHealth *NodeHealth) (healthy bool, err error) {
	nodeHealth.Update()
	healthy, err = WriteRegisterNode(nodeHealth)
	atomic.StoreInt64(&lastHealthCheckUnixNano, time.Now().UnixNano())
	if healthy {
		atomic.StoreInt64(&lastGoodHealthCheckUnixNano, time.Now().UnixNano())
	}
	return healthy, err
}

// HealthTest attempts to write to the backend database and get a result
func HealthTest() (health *HealthStatus, err error) {
	cacheKey := util.ProcessToken.Hash
	if healthStatus, found := lastHealthCheckCache.Get(cacheKey); found {
		return healthStatus.(*HealthStatus), nil
	}

	health = &HealthStatus{Healthy: false, Hostname: ThisHostname, Token: util.ProcessToken.Hash}
	defer lastHealthCheckCache.Set(cacheKey, health, cache.DefaultExpiration)

	if healthy, err := RegisterNode(ThisNodeHealth); err != nil {
		health.Error = err
		return health, log.Errore(err)
	} else {
		health.Healthy = healthy
	}

	if orcraft.IsRaftEnabled() {
		health.ActiveNode.Hostname = orcraft.GetLeader()
		health.IsActiveNode = orcraft.IsLeader()
		health.RaftLeader = orcraft.GetLeader()
		health.RaftLeaderURI = orcraft.LeaderURI.Get()
		health.IsRaftLeader = orcraft.IsLeader()
		health.RaftAdvertise = config.Config.RaftAdvertise
		health.RaftHealthyMembers = orcraft.HealthyMembers()
	} else {
		if health.ActiveNode, health.IsActiveNode, err = ElectedNode(); err != nil {
			health.Error = err
			return health, log.Errore(err)
		}
	}
	health.AvailableNodes, err = ReadAvailableNodes(true)

	return health, nil
}

func SinceLastHealthCheck() time.Duration {
	timeNano := atomic.LoadInt64(&lastHealthCheckUnixNano)
	if timeNano == 0 {
		return 0
	}
	return time.Since(time.Unix(0, timeNano))
}

func SinceLastGoodHealthCheck() time.Duration {
	timeNano := atomic.LoadInt64(&lastGoodHealthCheckUnixNano)
	if timeNano == 0 {
		return 0
	}
	return time.Since(time.Unix(0, timeNano))
}

// ContinuousRegistration will continuously update the node_health
// table showing that the current process is still running.
func ContinuousRegistration(extraInfo string, command string) {
	ThisNodeHealth.ExtraInfo = extraInfo
	ThisNodeHealth.Command = command
	continuousRegistrationOnce.Do(func() {
		tickOperation := func() {
			healthy, err := RegisterNode(ThisNodeHealth)
			if err != nil {
				log.Errorf("ContinuousRegistration: RegisterNode failed: %+v", err)
			}
			if healthy {
				atomic.StoreInt64(&LastContinousCheckHealthy, 1)
			} else {
				atomic.StoreInt64(&LastContinousCheckHealthy, 0)
			}
		}
		// First one is synchronous
		tickOperation()
		go func() {
			registrationTick := time.Tick(config.HealthPollSeconds * time.Second)
			for range registrationTick {
				// We already run inside a go-routine so
				// do not do this asynchronously.  If we
				// get stuck then we don't want to fill up
				// the backend pool with connections running
				// this maintenance operation.
				tickOperation()
			}
		}()
	})
}

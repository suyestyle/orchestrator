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
Package metrics 提供 Orchestrator 的监控指标收集和报告功能。

该包实现了以下核心功能：

1. 指标收集框架
   - 定期指标收集机制
   - 可插拔的指标收集器
   - 异步指标处理
   - 内存高效的指标管理

2. 多种指标后端支持
   - Graphite 集成（graphite.go）
   - Prometheus 集成（prometheus.go）
   - 自定义指标后端
   - 指标格式转换

3. 性能监控
   - 拓扑发现性能
   - 故障恢复时间
   - API 响应时间
   - 数据库操作延迟

4. 业务指标
   - 集群数量统计
   - 实例状态分布
   - 恢复操作统计
   - 错误率监控

5. 系统指标
   - 内存使用情况
   - 连接池状态
   - 缓存命中率
   - 并发操作数量

该模块为 Orchestrator 提供了全面的可观测性支持，帮助运维人员监控
系统健康状态和性能表现。
*/
package metrics

import (
	"github.com/openark/orchestrator/go/config"
	"time"
)

// matricTickCallbacks 存储所有注册的指标回调函数
// 这些函数会在每个指标收集周期被调用
var matricTickCallbacks [](func())

// InitMetrics 初始化指标收集系统
// 该函数在应用程序生命周期中只调用一次，在配置加载后执行
//
// 功能：
// 1. 启动指标收集的定时器
// 2. 定期执行所有注册的指标回调函数
// 3. 使用异步方式避免阻塞主流程
//
// 返回初始化过程中的错误（当前总是返回 nil）
func InitMetrics() error {
	// 启动指标收集的后台 goroutine
	go func() {
		// 创建定时器，间隔时间由配置文件指定
		metricsCallbackTick := time.Tick(time.Duration(config.DebugMetricsIntervalSeconds) * time.Second)

		// 定期执行指标收集
		for range metricsCallbackTick {
			// 并发执行所有注册的回调函数，避免单个慢回调阻塞其他回调
			for _, f := range matricTickCallbacks {
				go f()
			}
		}
	}()

	return nil
}

// OnMetricsTick 注册一个指标收集回调函数
// 注册的函数会在每个指标收集周期被异步调用
//
// 参数：
//   f: 要注册的回调函数，该函数应该收集并报告特定的指标
//
// 使用示例：
//   OnMetricsTick(func() {
//       // 收集并报告自定义指标
//       reportInstanceCount()
//   })
func OnMetricsTick(f func()) {
	matricTickCallbacks = append(matricTickCallbacks, f)
}

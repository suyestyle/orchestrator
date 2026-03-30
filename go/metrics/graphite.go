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
graphite.go 实现了 Orchestrator 与 Graphite 监控系统的集成。

主要功能：
1. 配置 Graphite 连接和路径设置
2. 定期向 Graphite 发送指标数据
3. 主机名格式化和路径处理
4. 连接错误处理和重连机制

Graphite 是一个流行的时间序列数据库，常用于系统监控和性能分析。
该模块使 Orchestrator 能够将关键指标发送到 Graphite，便于：
- 长期性能趋势分析
- 告警和仪表板创建
- 与其他监控系统集成
- 历史数据查询和可视化
*/
package metrics

import (
	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/config"
	"github.com/openark/orchestrator/go/process"
	"github.com/rcrowley/go-metrics"
	"net"
	"strings"
	"time"
)

// InitGraphiteMetrics 初始化 Graphite 指标报告功能
// 该函数在应用程序生命周期中只调用一次，在配置加载后执行
//
// 功能：
// 1. 验证 Graphite 配置的有效性
// 2. 建立与 Graphite 服务器的连接
// 3. 配置指标路径和报告间隔
// 4. 启动定期指标发送机制
//
// 配置要求：
// - GraphiteAddr: Graphite 服务器地址
// - GraphitePollSeconds: 指标发送间隔（秒）
// - GraphitePath: 指标路径前缀
//
// 返回初始化过程中的错误
func InitGraphiteMetrics() error {
	// 检查是否配置了 Graphite 服务器地址
	if config.Config.GraphiteAddr == "" {
		return nil // 未配置，跳过 Graphite 集成
	}

	// 检查指标发送间隔配置
	if config.Config.GraphitePollSeconds <= 0 {
		return nil // 无效间隔，跳过 Graphite 集成
	}

	// 检查指标路径配置
	if config.Config.GraphitePath == "" {
		return log.Errorf("No graphite path provided (see GraphitePath config variable). Will not log to graphite")
	}
	addr, err := net.ResolveTCPAddr("tcp", config.Config.GraphiteAddr)
	if err != nil {
		return log.Errore(err)
	}
	graphitePathHostname := process.ThisHostname
	if config.Config.GraphiteConvertHostnameDotsToUnderscores {
		graphitePathHostname = strings.Replace(graphitePathHostname, ".", "_", -1)
	}
	graphitePath := config.Config.GraphitePath
	graphitePath = strings.Replace(graphitePath, "{hostname}", graphitePathHostname, -1)

	log.Debugf("Will log to graphite on %+v, %+v", config.Config.GraphiteAddr, graphitePath)

	go graphite.Graphite(metrics.DefaultRegistry, time.Duration(config.Config.GraphitePollSeconds)*time.Second, graphitePath, addr)

	return nil
}

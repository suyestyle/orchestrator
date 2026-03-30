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
Package main 是 Orchestrator 的主程序入口点。

Orchestrator 是一个 MySQL 高可用和复制管理工具，支持以下运行模式：
1. CLI 模式：通过命令行执行各种管理操作
2. HTTP 模式：提供 Web UI 和 API 服务

主要功能包括：
- MySQL 拓扑结构自动发现和可视化
- 故障检测和自动恢复
- 主从复制关系管理和重构
- 集群状态监控和告警

该程序支持多种数据库后端（MySQL、SQLite）用于存储拓扑信息和元数据。
*/
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/app"
	"github.com/openark/orchestrator/go/config"
	"github.com/openark/orchestrator/go/inst"

	// MySQL 和 SQLite 数据库驱动
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// AppVersion 应用版本号，编译时注入
var AppVersion, GitCommit string

// main 是应用程序的入口点，解析命令行参数并启动对应的服务模式
// 支持两种主要运行模式：
// 1. CLI 模式：执行命令行操作（默认模式）
// 2. HTTP 模式：启动 Web 服务和 API 服务器
func main() {
	// 基本配置参数
	configFile := flag.String("config", "", "配置文件路径")
	command := flag.String("c", "", "要执行的命令，必需参数。通过 'orchestrator -c help' 查看完整命令列表")
	strict := flag.Bool("strict", false, "严格模式（更多检查，较慢）")

	// MySQL 实例相关参数
	instance := flag.String("i", "", "MySQL 实例地址，格式：host_fqdn[:port]，例如 db.company.com:3306")
	sibling := flag.String("s", "", "兄弟实例地址，格式：host_fqdn[:port]")
	destination := flag.String("d", "", "目标实例地址，格式：host_fqdn[:port]（与 -s 同义）")

	// 操作元信息参数
	owner := flag.String("owner", "", "操作执行者")
	reason := flag.String("reason", "", "操作原因")
	duration := flag.String("duration", "", "维护窗口时长（格式：59s, 59m, 23h, 6d, 4w）")

	// 过滤和匹配参数
	pattern := flag.String("pattern", "", "正则表达式模式")
	clusterAlias := flag.String("alias", "", "集群别名")
	pool := flag.String("pool", "", "逻辑池名称（用于池相关命令）")
	hostnameFlag := flag.String("hostname", "", "主机名/FQDN/CNAME/VIP（用于主机名解析相关命令）")

	// 服务运行参数
	discovery := flag.Bool("discovery", true, "启用自动拓扑发现模式")

	// 日志级别参数
	quiet := flag.Bool("quiet", false, "静默模式")
	verbose := flag.Bool("verbose", false, "详细输出模式")
	debug := flag.Bool("debug", false, "调试模式（非常详细的输出）")
	stack := flag.Bool("stack", false, "错误时添加堆栈跟踪")
	// 高级运行时参数
	config.RuntimeCLIFlags.SkipBinlogSearch = flag.Bool("skip-binlog-search", false, "使用 Pseudo-GTID 匹配时仅使用中继日志，避免搜索不存在的 Pseudo-GTID 条目（如在有复制过滤器的服务器上）")
	config.RuntimeCLIFlags.SkipUnresolve = flag.Bool("skip-unresolve", false, "不执行主机名反解析")
	config.RuntimeCLIFlags.SkipUnresolveCheck = flag.Bool("skip-unresolve-check", false, "跳过检查反解析映射（通过 hostname_unresolve 表）是否解析回相同主机名")
	config.RuntimeCLIFlags.Noop = flag.Bool("noop", false, "空运行模式；不执行破坏性操作")
	config.RuntimeCLIFlags.BinlogFile = flag.String("binlog", "", "二进制日志文件名")
	config.RuntimeCLIFlags.Statement = flag.String("statement", "", "SQL 语句/提示")
	config.RuntimeCLIFlags.GrabElection = flag.Bool("grab-election", false, "获取领导权（仅适用于连续模式）")
	config.RuntimeCLIFlags.PromotionRule = flag.String("promotion-rule", "prefer", "候选者提升规则 (prefer|neutral|prefer_not|must_not)")
	config.RuntimeCLIFlags.Version = flag.Bool("version", false, "打印版本信息并退出")
	config.RuntimeCLIFlags.SkipContinuousRegistration = flag.Bool("skip-continuous-registration", false, "跳过 CLI 命令的连续注册（减少后端数据库负载）")
	config.RuntimeCLIFlags.EnableDatabaseUpdate = flag.Bool("enable-database-update", false, "启用数据库更新，覆盖 SkipOrchestratorDatabaseUpdate 配置")
	config.RuntimeCLIFlags.IgnoreRaftSetup = flag.Bool("ignore-raft-setup", false, "CLI 调用时忽略 RaftEnabled 配置（CLI 默认不允许在 raft 设置中使用）。注意：CLI 操作可能不会反映到所有 raft 节点")
	config.RuntimeCLIFlags.Tag = flag.String("tag", "", "要添加的标签（'tagname' 或 'tagname=tagvalue'）或搜索标签（'tagname' 或 'tagname=tagvalue' 或逗号分隔的 'tag0,tag1=val1,tag2' 表示所有标签的交集）")

	// 解析命令行参数
	flag.Parse()

	// 参数验证和预处理
	if *destination != "" && *sibling != "" {
		log.Fatalf("-s 和 -d 是同义词，但两个都被指定了。您可能正在做错误的操作。")
	}

	// 验证提升规则参数
	switch *config.RuntimeCLIFlags.PromotionRule {
	case "prefer", "neutral", "prefer_not", "must_not":
		{
			// 有效的提升规则
		}
	default:
		{
			log.Fatalf("-promotion-rule 仅支持 prefer|neutral|prefer_not|must_not")
		}
	}

	// 设置目标实例，优先使用 destination
	if *destination == "" {
		*destination = *sibling
	}

	// 设置日志级别，默认为 ERROR 级别
	log.SetLevel(log.ERROR)
	if *verbose {
		log.SetLevel(log.INFO) // 详细模式
	}
	if *debug {
		log.SetLevel(log.DEBUG) // 调试模式，最详细
	}
	if *stack {
		log.SetPrintStackTrace(*stack) // 启用堆栈跟踪
	}

	// 处理版本信息请求
	if *config.RuntimeCLIFlags.Version {
		fmt.Println(AppVersion)
		fmt.Println(GitCommit)
		return
	}

	// 构建启动信息
	startText := "starting orchestrator"
	if AppVersion != "" {
		startText += ", version: " + AppVersion
	}
	if GitCommit != "" {
		startText += ", git commit: " + GitCommit
	}
	log.Info(startText)

	// 读取配置文件
	if len(*configFile) > 0 {
		// 强制读取指定的配置文件
		config.ForceRead(*configFile)
	} else {
		// 按优先级顺序读取默认配置文件位置
		config.Read("/etc/orchestrator.conf.json", "conf/orchestrator.conf.json", "orchestrator.conf.json")
	}

	// 处理数据库更新覆盖配置
	if *config.RuntimeCLIFlags.EnableDatabaseUpdate {
		config.Config.SkipOrchestratorDatabaseUpdate = false
	}

	// 根据配置文件中的调试设置调整日志级别
	if config.Config.Debug {
		log.SetLevel(log.DEBUG)
	}

	// 静默模式覆盖所有其他日志级别设置
	if *quiet {
		log.SetLevel(log.ERROR)
	}

	// 配置系统日志支持
	if config.Config.EnableSyslog {
		log.EnableSyslogWriter("orchestrator")
		log.SetSyslogLevel(log.INFO)
	}

	// 配置审计日志到系统日志
	if config.Config.AuditToSyslog {
		inst.EnableAuditSyslog()
	}

	// 设置运行时版本信息并标记配置已加载
	config.RuntimeCLIFlags.ConfiguredVersion = AppVersion
	config.MarkConfigurationLoaded()

	// 检查是否没有提供任何命令或参数
	if len(flag.Args()) == 0 && *command == "" {
		// 无命令，无参数：仅显示提示信息
		fmt.Println(app.AppPrompt)
		return
	}

	// 处理帮助命令的特殊逻辑
	helpTopic := ""
	if flag.Arg(0) == "help" {
		if flag.Arg(1) != "" {
			helpTopic = flag.Arg(1)
		}
		if helpTopic == "" {
			helpTopic = *command
		}
		if helpTopic == "" {
			// 巧妙的方式让 CLI 生效，就像用户输入了 `orchestrator -c help cli`
			*command = "help"
			flag.Args()[0] = "cli"
		}
	}

	// 根据参数选择运行模式
	switch {
	case helpTopic != "":
		// 显示帮助信息
		app.HelpCommand(helpTopic)
	case len(flag.Args()) == 0 || flag.Arg(0) == "cli":
		// CLI 模式：执行命令行操作
		app.CliWrapper(*command, *strict, *instance, *destination, *owner, *reason, *duration, *pattern, *clusterAlias, *pool, *hostnameFlag)
	case flag.Arg(0) == "http":
		// HTTP 模式：启动 Web 服务器和 API
		app.Http(*discovery)
	default:
		// 无效参数，显示使用说明
		fmt.Fprintln(os.Stderr, `Usage:
  orchestrator --options... [cli|http]
See complete list of commands:
  orchestrator -c help
Full blown documentation:
  orchestrator`)
		os.Exit(1)
	}
}

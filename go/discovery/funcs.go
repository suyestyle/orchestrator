/*
   Copyright 2017 Simon J Mudd

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
funcs.go 提供发现模块的统计和数学计算辅助函数。

该文件包含用于分析发现性能指标的工具函数，支持：
1. 平均值计算
2. 百分位数计算
3. 性能数据分析
4. 统计错误处理

这些函数主要用于发现过程的性能监控和分析，帮助：
- 监控发现操作的延迟分布
- 计算发现成功率
- 分析网络延迟统计
- 生成性能报告

所有函数都提供了错误处理，确保在数据异常时返回安全的默认值。
*/
package discovery

import (
	"github.com/montanaflynn/stats"
)

// mean 计算数据集的平均值
// 这是一个内部工具函数，用于安全地计算平均值
//
// 参数：
//   values: 要计算平均值的浮点数数据集
//
// 返回：
//   计算得到的平均值，如果计算失败则返回 0
//
// 用途：
//   主要用于计算发现操作的平均延迟、成功率等指标
func mean(values stats.Float64Data) float64 {
	s, err := stats.Mean(values)
	if err != nil {
		return 0 // 计算失败时返回安全默认值
	}
	return s
}

// percentile 计算指定百分位数的值
// 这是一个内部工具函数，用于安全地计算百分位数
//
// 参数：
//   values: 要分析的浮点数数据集
//   percent: 要计算的百分位数（0-100）
//
// 返回：
//   指定百分位数的值，如果计算失败则返回 0
//
// 用途：
//   用于分析发现操作的延迟分布，如 P50、P95、P99 等关键指标
//   帮助识别性能异常和系统瓶颈
func percentile(values stats.Float64Data, percent float64) float64 {
	s, err := stats.Percentile(values, percent)
	if err != nil {
		return 0 // 计算失败时返回安全默认值
	}
	return s
}

// internal routine to return the maximum value or 0
func max(values stats.Float64Data) float64 {
	s, err := stats.Max(values)
	if err != nil {
		return 0
	}
	return s
}

// internal routine to return the minimum value or 9e9
func min(values stats.Float64Data) float64 {
	s, err := stats.Min(values)
	if err != nil {
		return 9e9 // a large number (should use something better than this but it's ok for now)
	}
	return s
}

// internal routine to return the median or 0
func median(values stats.Float64Data) float64 {
	s, err := stats.Median(values)
	if err != nil {
		return 0
	}
	return s
}

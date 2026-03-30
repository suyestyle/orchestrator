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
Package discovery 管理 MySQL 实例的自动发现功能。

该包提供了以下核心功能：

1. 发现请求队列管理
   - 有序队列，无重复元素
   - 非阻塞的 push() 操作
   - 阻塞式的 pop() 操作（空队列时）
   - 并发安全的队列操作

2. 拓扑自动发现
   - 递归发现 MySQL 复制拓扑
   - 基于 SHOW SLAVE HOSTS 和 PROCESSLIST
   - 自动检测新实例和拓扑变化
   - 智能去重和缓存机制

3. 发现策略优化
   - 并发发现控制
   - 发现优先级管理
   - 错误重试和退避机制
   - 资源使用限制

4. 指标收集和监控
   - 发现队列状态监控
   - 发现性能指标
   - 错误率统计
   - 发现延迟跟踪

5. 配置驱动的发现
   - 发现间隔配置
   - 并发度控制
   - 超时时间设置
   - 黑名单和白名单

该模块是 Orchestrator 拓扑感知能力的基础，确保系统能够
及时发现和跟踪 MySQL 环境的变化。
*/
package discovery

import (
	"sync"
	"time"

	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/config"
	"github.com/openark/orchestrator/go/inst"
)

// QueueMetric 包含队列的活跃和排队大小信息
// 用于监控发现队列的状态和性能
type QueueMetric struct {
	Active int // 当前正在处理的发现请求数量
	Queued int // 排队等待处理的发现请求数量
}

// Queue 包含管理发现请求的信息
// 实现了一个线程安全的、无重复的有序队列
type Queue struct {
	sync.Mutex // 保护并发访问的互斥锁

	name       string                        // 队列名称，用于日志和监控
	done       chan struct{}                 // 用于通知队列关闭的通道
	queue      chan inst.InstanceKey         // 发现请求队列
	queuedKeys map[inst.InstanceKey]time.Time // 跟踪排队的实例键和时间戳，用于去重
	consumedKeys map[inst.InstanceKey]time.Time
	metrics      []QueueMetric
}

// DiscoveryQueue contains the discovery queue which can then be accessed via an API call for monitoring.
// Currently this is accessed by ContinuousDiscovery() but also from http api calls.
// I may need to protect this better?
var discoveryQueue map[string](*Queue)
var dcLock sync.Mutex

func init() {
	discoveryQueue = make(map[string](*Queue))
}

// StopMonitoring stops monitoring all the queues
func StopMonitoring() {
	for _, q := range discoveryQueue {
		q.stopMonitoring()
	}
}

// CreateOrReturnQueue allows for creation of a new discovery queue or
// returning a pointer to an existing one given the name.
func CreateOrReturnQueue(name string) *Queue {
	dcLock.Lock()
	defer dcLock.Unlock()
	if q, found := discoveryQueue[name]; found {
		return q
	}

	q := &Queue{
		name:         name,
		queuedKeys:   make(map[inst.InstanceKey]time.Time),
		consumedKeys: make(map[inst.InstanceKey]time.Time),
		queue:        make(chan inst.InstanceKey, config.Config.DiscoveryQueueCapacity),
	}
	go q.startMonitoring()

	discoveryQueue[name] = q

	return q
}

// monitoring queue sizes until we are told to stop
func (q *Queue) startMonitoring() {
	log.Debugf("Queue.startMonitoring(%s)", q.name)
	ticker := time.NewTicker(time.Second) // hard-coded at every second

	for {
		select {
		case <-ticker.C: // do the periodic expiry
			q.collectStatistics()
		case <-q.done:
			return
		}
	}
}

// Stop monitoring the queue
func (q *Queue) stopMonitoring() {
	q.done <- struct{}{}
}

// do a check of the entries in the queue, both those active and queued
func (q *Queue) collectStatistics() {
	q.Lock()
	defer q.Unlock()

	q.metrics = append(q.metrics, QueueMetric{Queued: len(q.queuedKeys), Active: len(q.consumedKeys)})

	// remove old entries if we get too big
	if len(q.metrics) > config.Config.DiscoveryQueueMaxStatisticsSize {
		q.metrics = q.metrics[len(q.metrics)-config.Config.DiscoveryQueueMaxStatisticsSize:]
	}
}

// QueueLen returns the length of the queue (channel size + queued size)
func (q *Queue) QueueLen() int {
	q.Lock()
	defer q.Unlock()

	return len(q.queue) + len(q.queuedKeys)
}

// Push enqueues a key if it is not on a queue and is not being
// processed; silently returns otherwise.
func (q *Queue) Push(key inst.InstanceKey) {
	q.Lock()
	defer q.Unlock()

	// is it enqueued already?
	if _, found := q.queuedKeys[key]; found {
		return
	}

	// is it being processed now?
	if _, found := q.consumedKeys[key]; found {
		return
	}

	q.queuedKeys[key] = time.Now()
	q.queue <- key
}

// Consume fetches a key to process; blocks if queue is empty.
// Release must be called once after Consume.
func (q *Queue) Consume() inst.InstanceKey {
	q.Lock()
	queue := q.queue
	q.Unlock()

	key := <-queue

	q.Lock()
	defer q.Unlock()

	// alarm if have been waiting for too long
	timeOnQueue := time.Since(q.queuedKeys[key])
	if timeOnQueue > time.Duration(config.Config.InstancePollSeconds)*time.Second {
		log.Warningf("key %v spent %.4fs waiting on a discoveryQueue", key, timeOnQueue.Seconds())
	}

	q.consumedKeys[key] = q.queuedKeys[key]

	delete(q.queuedKeys, key)

	return key
}

// Release removes a key from a list of being processed keys
// which allows that key to be pushed into the queue again.
func (q *Queue) Release(key inst.InstanceKey) {
	q.Lock()
	defer q.Unlock()

	delete(q.consumedKeys, key)
}

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
Package util 提供 Orchestrator 的通用工具函数。

该包包含以下实用功能：

1. 令牌生成和管理
   - 安全随机令牌生成
   - SHA256 哈希计算
   - 短令牌生成（用于标识）
   - 时间戳结合的唯一性保证

2. 字符串处理工具
   - 字符串格式化
   - 编码转换
   - 安全字符串操作

3. 加密和哈希工具
   - 加密安全的随机数生成
   - 哈希值计算和验证
   - 数据完整性检查

4. 缓存和标识符
   - 唯一标识符生成
   - 缓存键生成
   - 会话令牌管理

该模块为 Orchestrator 的各个组件提供了基础的工具函数，
确保代码的复用性和一致性。所有加密相关的操作都使用了
加密安全的随机数生成器和标准的哈希算法。
*/
package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// shortTokenLength 短令牌的标准长度
	// 用于生成简短的标识符，适合日志显示和用户界面
	shortTokenLength = 8
)

// toHash 计算输入数据的 SHA256 哈希值
// 返回十六进制编码的哈希字符串
//
// 参数：
//   input: 要计算哈希的字节数据
//
// 返回：
//   十六进制编码的 SHA256 哈希字符串
//
// 安全性：
//   使用 SHA256 算法确保哈希的安全性和唯一性
func toHash(input []byte) string {
	hasher := sha256.New()
	hasher.Write(input)
	return hex.EncodeToString(hasher.Sum(nil))
}

// getRandomData 生成加密安全的随机字节数据
// 使用系统的加密安全随机数生成器
//
// 返回：
//   64 字节的随机数据
//
// 安全性：
//   使用 crypto/rand 包确保生成的随机数据具有加密安全强度
//   适用于生成令牌、密钥等安全敏感的数据
func getRandomData() []byte {
	size := 64 // 64 字节提供足够的熵
	rb := make([]byte, size)
	_, _ = rand.Read(rb) // 忽略错误，crypto/rand 在正常系统上不会失败
	return rb
}

func RandomHash() string {
	return toHash(getRandomData())
}

// Token is used to identify and validate requests to this service
type Token struct {
	Hash string
}

func (this *Token) Short() string {
	if len(this.Hash) <= shortTokenLength {
		return this.Hash
	}
	return this.Hash[0:shortTokenLength]
}

var ProcessToken *Token = NewToken()

func NewToken() *Token {
	return &Token{
		Hash: RandomHash(),
	}
}

func PrettyUniqueToken() string {
	return fmt.Sprintf("%d:%s", time.Now().UnixNano(), NewToken().Hash)
}

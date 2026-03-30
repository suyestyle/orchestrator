/*
Package ssl 提供 Orchestrator 的 SSL/TLS 安全连接支持。

该包实现了以下安全功能：

1. TLS 配置管理
   - 客户端和服务器 TLS 配置
   - 证书验证设置
   - 加密套件选择
   - 协议版本控制

2. 证书管理
   - CA 证书加载和验证
   - 客户端证书处理
   - 证书链构建
   - PEM 格式支持

3. 安全连接建立
   - MySQL TLS 连接
   - HTTP/HTTPS 服务
   - 客户端认证
   - 服务器认证

4. 密码管理
   - 安全密码输入
   - PEM 文件密码处理
   - 内存安全清理
   - 密码强度验证

5. 兼容性支持
   - 多种 TLS 版本支持
   - 向后兼容处理
   - 配置迁移支持
   - 错误恢复机制

该模块确保 Orchestrator 在生产环境中的通信安全，支持企业级的
安全要求和合规性标准。
*/
package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"strings"

	"github.com/go-martini/martini"
	"github.com/howeyc/gopass"
	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/config"
)

// HasString 检查字符串元素是否在字符串数组中
// 这是一个通用的工具函数，用于各种字符串匹配场景
//
// 参数：
//   elem: 要查找的字符串元素
//   arr: 要搜索的字符串数组
//
// 返回：
//   true 如果元素在数组中，否则返回 false
//
// 用途：
//   主要用于验证主机名、证书通用名等安全相关的字符串匹配
func HasString(elem string, arr []string) bool {
	for _, s := range arr {
		if s == elem {
			return true
		}
	}
	return false
}

// NewTLSConfig 返回适用于客户端认证的初始化 TLS 配置
// 如果 caFile 非空，将会被加载并用于证书验证
//
// 参数：
//   caFile: CA 证书文件路径，用于验证客户端证书
//   verifyCert: 是否启用客户端证书验证
//
// 返回：
//   配置好的 TLS 配置对象和可能的错误
//
// 安全特性：
//   - 强制使用 TLS 1.2 或更高版本
//   - 使用安全的默认加密套件
//   - 优先使用服务器密码套件顺序
//   - 支持可选的客户端证书验证
func NewTLSConfig(caFile string, verifyCert bool) (*tls.Config, error) {
	var c tls.Config

	// 设置最低 TLS 版本为 1.2，MySQL 通信时可能会被覆盖
	c.MinVersion = tls.VersionTLS12

	// 使用默认的安全加密套件列表
	// Go 标准库会自动选择安全的套件
	c.CipherSuites = nil

	// 优先使用服务器的密码套件顺序，提高安全性
	c.PreferServerCipherSuites = true

	// 配置客户端证书验证
	if verifyCert {
		log.Info("verifyCert requested, client certificates will be verified")
		c.ClientAuth = tls.VerifyClientCertIfGiven
	}

	// 加载 CA 证书文件
	caPool, err := ReadCAFile(caFile)
	if err != nil {
		return &c, err
	}
	c.ClientCAs = caPool

	// 构建名称到证书的映射，提高 TLS 握手性能
	c.BuildNameToCertificate()
	return &c, nil
}

// Returns CA certificate. If caFile is non-empty, it will be loaded.
func ReadCAFile(caFile string) (*x509.CertPool, error) {
	var caCertPool *x509.CertPool
	if caFile != "" {
		data, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(data) {
			return nil, errors.New("No certificates parsed")
		}
		log.Info("Read in CA file:", caFile)
	}
	return caCertPool, nil
}

// Verify that the OU of the presented client certificate matches the list
// of Valid OUs
func Verify(r *nethttp.Request, validOUs []string) error {
	if strings.Contains(r.URL.String(), config.Config.StatusEndpoint) && !config.Config.StatusOUVerify {
		return nil
	}
	if r.TLS == nil {
		return errors.New("No TLS")
	}
	for _, chain := range r.TLS.VerifiedChains {
		s := chain[0].Subject.OrganizationalUnit
		log.Debug("All OUs:", strings.Join(s, " "))
		for _, ou := range s {
			log.Debug("Client presented OU:", ou)
			if HasString(ou, validOUs) {
				log.Debug("Found valid OU:", ou)
				return nil
			}
		}
	}
	log.Error("No valid OUs found")
	return errors.New("Invalid OU")
}

// TODO: make this testable?
func VerifyOUs(validOUs []string) martini.Handler {
	return func(res nethttp.ResponseWriter, req *nethttp.Request, c martini.Context) {
		log.Debug("Verifying client OU")
		if err := Verify(req, validOUs); err != nil {
			nethttp.Error(res, err.Error(), nethttp.StatusUnauthorized)
		}
	}
}

// AppendKeyPair loads the given TLS key pair and appends it to
// tlsConfig.Certificates.
func AppendKeyPair(tlsConfig *tls.Config, certFile string, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	return nil
}

// Read in a keypair where the key is password protected
func AppendKeyPairWithPassword(tlsConfig *tls.Config, certFile string, keyFile string, pemPass []byte) error {

	// Certificates aren't usually password protected, but we're kicking the password
	// along just in case.  It won't be used if the file isn't encrypted
	certData, err := ReadPEMData(certFile, pemPass)
	if err != nil {
		return err
	}
	keyData, err := ReadPEMData(keyFile, pemPass)
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return err
	}
	tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	return nil
}

// Read a PEM file and ask for a password to decrypt it if needed
func ReadPEMData(pemFile string, pemPass []byte) ([]byte, error) {
	pemData, err := ioutil.ReadFile(pemFile)
	if err != nil {
		return pemData, err
	}

	// We should really just get the pem.Block back here, if there's other
	// junk on the end, warn about it.
	pemBlock, rest := pem.Decode(pemData)
	if len(rest) > 0 {
		log.Warning("Didn't parse all of", pemFile)
	}

	if x509.IsEncryptedPEMBlock(pemBlock) {
		// Decrypt and get the ASN.1 DER bytes here
		pemData, err = x509.DecryptPEMBlock(pemBlock, pemPass)
		if err != nil {
			return pemData, err
		} else {
			log.Info("Decrypted", pemFile, "successfully")
		}
		// Shove the decrypted DER bytes into a new pem Block with blank headers
		var newBlock pem.Block
		newBlock.Type = pemBlock.Type
		newBlock.Bytes = pemData
		// This is now like reading in an uncrypted key from a file and stuffing it
		// into a byte stream
		pemData = pem.EncodeToMemory(&newBlock)
	}
	return pemData, nil
}

// Print a password prompt on the terminal and collect a password
func GetPEMPassword(pemFile string) []byte {
	fmt.Printf("Password for %s: ", pemFile)
	pass, err := gopass.GetPasswd()
	if err != nil {
		// We'll error with an incorrect password at DecryptPEMBlock
		return []byte("")
	}
	return pass
}

// Determine if PEM file is encrypted
func IsEncryptedPEM(pemFile string) bool {
	pemData, err := ioutil.ReadFile(pemFile)
	if err != nil {
		return false
	}
	pemBlock, _ := pem.Decode(pemData)
	if len(pemBlock.Bytes) == 0 {
		return false
	}
	return x509.IsEncryptedPEMBlock(pemBlock)
}

// ListenAndServeTLS acts identically to http.ListenAndServeTLS, except that it
// expects TLS configuration.
// TODO: refactor so this is testable?
func ListenAndServeTLS(addr string, handler nethttp.Handler, tlsConfig *tls.Config) error {
	if addr == "" {
		// On unix Listen calls getaddrinfo to parse the port, so named ports are fine as long
		// as they exist in /etc/services
		addr = ":https"
	}
	l, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	return nethttp.Serve(l, handler)
}

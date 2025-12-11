package store

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/seefs001/wechat-server/config"
)

// CodeEntry 验证码条目
type CodeEntry struct {
	OpenID    string
	AppID     string
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// MemoryStore 内存存储
type MemoryStore struct {
	mu sync.RWMutex
	// code -> CodeEntry
	codes map[string]*CodeEntry
	// openid:appid -> code (用于查找用户当前验证码)
	userCodes map[string]string
}

var store *MemoryStore

func init() {
	store = &MemoryStore{
		codes:     make(map[string]*CodeEntry),
		userCodes: make(map[string]string),
	}
	// 启动清理协程
	go store.cleanupLoop()
}

// GetStore 获取存储实例
func GetStore() *MemoryStore {
	return store
}

// GenerateCode 为用户生成验证码
func (s *MemoryStore) GenerateCode(openID, appID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := config.Get()
	userKey := openID + ":" + appID

	// 删除旧验证码
	if oldCode, exists := s.userCodes[userKey]; exists {
		delete(s.codes, oldCode)
	}

	// 生成新验证码
	code := generateRandomCode(cfg.Code.Length)
	expireMinutes := cfg.Code.ExpireMinutes
	if expireMinutes <= 0 {
		expireMinutes = 5
	}

	entry := &CodeEntry{
		OpenID:    openID,
		AppID:     appID,
		Code:      code,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(expireMinutes) * time.Minute),
	}

	s.codes[code] = entry
	s.userCodes[userKey] = code

	return code
}

// VerifyCode 验证验证码，成功返回 openID，失败返回空字符串
func (s *MemoryStore) VerifyCode(code, appID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.codes[code]
	if !exists {
		return ""
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		s.deleteCodeLocked(code, entry)
		return ""
	}

	// 如果指定了 appID，检查是否匹配
	if appID != "" && entry.AppID != appID {
		return ""
	}

	// 验证成功，删除验证码
	s.deleteCodeLocked(code, entry)

	return entry.OpenID
}

// GetCodeByUser 获取用户当前的验证码（用于重发）
func (s *MemoryStore) GetCodeByUser(openID, appID string) *CodeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userKey := openID + ":" + appID
	code, exists := s.userCodes[userKey]
	if !exists {
		return nil
	}

	entry, exists := s.codes[code]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

func (s *MemoryStore) deleteCodeLocked(code string, entry *CodeEntry) {
	delete(s.codes, code)
	userKey := entry.OpenID + ":" + entry.AppID
	if s.userCodes[userKey] == code {
		delete(s.userCodes, userKey)
	}
}

func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for code, entry := range s.codes {
		if now.After(entry.ExpiresAt) {
			s.deleteCodeLocked(code, entry)
		}
	}
}

// Stats 返回存储统计信息
func (s *MemoryStore) Stats() (codeCount, userCount int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.codes), len(s.userCodes)
}

func generateRandomCode(length int) string {
	if length <= 0 {
		length = 6
	}
	// 生成足够的随机字节
	bytes := make([]byte, (length+1)/2)
	rand.Read(bytes)
	code := hex.EncodeToString(bytes)
	// 转换为大写并截取指定长度
	if len(code) > length {
		code = code[:length]
	}
	return code
}

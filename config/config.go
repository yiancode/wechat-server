package config

import (
	"os"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config 全局配置
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Accounts []WechatAccount `yaml:"accounts"`
	Code     CodeConfig      `yaml:"code"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port     int    `yaml:"port"`
	APIToken string `yaml:"api_token"`
}

// WechatAccount 微信公众号账户配置
type WechatAccount struct {
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
	Token     string `yaml:"token"`
	Name      string `yaml:"name"`
}

// CodeConfig 验证码配置
type CodeConfig struct {
	Length        int `yaml:"length"`
	ExpireMinutes int `yaml:"expire_minutes"`
}

var (
	cfg  *Config
	once sync.Once
)

// Load 加载配置
func Load() (*Config, error) {
	var err error
	once.Do(func() {
		cfg = &Config{
			Server: ServerConfig{
				Port:     3000,
				APIToken: "",
			},
			Code: CodeConfig{
				Length:        6,
				ExpireMinutes: 5,
			},
		}

		// 尝试从配置文件加载
		configPath := getEnv("CONFIG_PATH", "config.yaml")
		if data, readErr := os.ReadFile(configPath); readErr == nil {
			if parseErr := yaml.Unmarshal(data, cfg); parseErr != nil {
				err = parseErr
				return
			}
		}

		// 环境变量覆盖
		if port := os.Getenv("PORT"); port != "" {
			if p, parseErr := strconv.Atoi(port); parseErr == nil {
				cfg.Server.Port = p
			}
		}
		if token := os.Getenv("API_TOKEN"); token != "" {
			cfg.Server.APIToken = token
		}
		if length := os.Getenv("CODE_LENGTH"); length != "" {
			if l, parseErr := strconv.Atoi(length); parseErr == nil {
				cfg.Code.Length = l
			}
		}
		if expire := os.Getenv("CODE_EXPIRE_MINUTES"); expire != "" {
			if e, parseErr := strconv.Atoi(expire); parseErr == nil {
				cfg.Code.ExpireMinutes = e
			}
		}

		// 从环境变量加载单个公众号配置（向后兼容）
		if appID := os.Getenv("WECHAT_APPID"); appID != "" {
			account := WechatAccount{
				AppID:     appID,
				AppSecret: os.Getenv("WECHAT_SECRET"),
				Token:     os.Getenv("WECHAT_TOKEN"),
				Name:      os.Getenv("WECHAT_NAME"),
			}
			// 检查是否已存在
			exists := false
			for i, acc := range cfg.Accounts {
				if acc.AppID == appID {
					cfg.Accounts[i] = account
					exists = true
					break
				}
			}
			if !exists {
				cfg.Accounts = append(cfg.Accounts, account)
			}
		}
	})

	return cfg, err
}

// Get 获取配置（必须先调用 Load）
func Get() *Config {
	if cfg == nil {
		Load()
	}
	return cfg
}

// GetAccountByAppID 根据 AppID 获取公众号配置
func GetAccountByAppID(appID string) *WechatAccount {
	for _, acc := range Get().Accounts {
		if acc.AppID == appID {
			return &acc
		}
	}
	return nil
}

// GetAccountByToken 根据 Token 获取公众号配置
func GetAccountByToken(token string) *WechatAccount {
	for _, acc := range Get().Accounts {
		if acc.Token == token {
			return &acc
		}
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

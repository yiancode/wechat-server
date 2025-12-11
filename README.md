# WeChat Server

微信公众号验证码登录服务，为 [Yi-API](https://github.com/QuantumNous/new-api) 提供微信登录支持。

## 功能特性

- 支持多公众号管理
- 验证码自动生成与过期
- 兼容 Yi-API / One API 的微信登录接口
- Docker 一键部署
- 最小依赖，轻量高效

## 快速开始

### Docker 部署（推荐）

1. 创建配置文件：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，填入你的公众号配置
```

2. 启动服务：

```bash
docker-compose up -d
```

### 手动部署

```bash
# 下载依赖
go mod download

# 运行
go run main.go

# 或编译后运行
go build -o wechat-server
./wechat-server
```

## 配置说明

### 多公众号模式（推荐）

使用 `config.yaml` 配置多个公众号：

```yaml
server:
  port: 3000
  api_token: "your-api-token"

accounts:
  - app_id: "wx1234567890"
    app_secret: "secret1"
    token: "token1"
    name: "公众号A"

  - app_id: "wx0987654321"
    app_secret: "secret2"
    token: "token2"
    name: "公众号B"

code:
  length: 6
  expire_minutes: 5
```

### 单公众号模式

使用环境变量配置：

```bash
export PORT=3000
export API_TOKEN=your-api-token
export WECHAT_APPID=wx1234567890
export WECHAT_SECRET=your-app-secret
export WECHAT_TOKEN=your-wechat-token
export WECHAT_NAME=我的公众号
```

## 微信公众号配置

1. 登录 [微信公众平台](https://mp.weixin.qq.com/)
2. 进入 **设置与开发 → 基本配置**
3. 配置服务器：
   - **URL**: `https://your-domain.com/wechat/{app_id}`
   - **Token**: 与配置文件中的 `token` 一致
   - **EncodingAESKey**: 随机生成（可选）
   - **消息加解密方式**: 明文模式

### 多公众号 URL 配置

每个公众号配置独立的服务器地址：

| 公众号 | 服务器 URL |
|--------|------------|
| 公众号A | `https://wechat.example.com/wechat/wx1234567890` |
| 公众号B | `https://wechat.example.com/wechat/wx0987654321` |

## API 接口

### 验证用户（Yi-API 调用）

```
GET /api/wechat/user?code={验证码}&app_id={公众号AppID}
Header: Authorization: {api_token}

成功响应:
{
    "success": true,
    "message": "",
    "data": "用户OpenID"
}

错误响应:
{
    "success": false,
    "message": "验证码错误或已过期",
    "data": ""
}
```

### 服务状态

```
GET /api/wechat/stats
Header: Authorization: {api_token}

响应:
{
    "success": true,
    "data": {
        "active_codes": 10,
        "active_users": 5,
        "accounts": [
            {"app_id": "wx123", "name": "公众号A"}
        ]
    }
}
```

### 健康检查

```
GET /health

响应:
{
    "status": "ok"
}
```

## 在 Yi-API 中配置

1. 登录 Yi-API 管理后台
2. 进入 **系统设置 → 配置 WeChat Server**
3. 填写：
   - **WeChat Server 服务器地址**: `https://your-domain.com`
   - **WeChat Server 访问凭证**: 配置文件中的 `api_token`
   - **微信公众号二维码图片链接**: 公众号二维码 URL
4. 保存设置
5. 在 **配置登录注册** 中开启微信登录

## 工作流程

```
┌─────────┐    扫码关注     ┌─────────────┐
│  用户   │ ──────────────→ │ 微信公众号  │
└─────────┘                 └─────────────┘
     │                            │
     │ 输入验证码                  │ 发送验证码
     ↓                            ↓
┌─────────┐   验证 code    ┌─────────────┐
│ Yi-API  │ ──────────────→│WeChat Server│
└─────────┘                └─────────────┘
     │                            │
     │←── 返回 OpenID ────────────┘
     │
     ↓
  登录/注册成功
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | 3000 |
| `API_TOKEN` | API 访问凭证 | - |
| `CONFIG_PATH` | 配置文件路径 | config.yaml |
| `WECHAT_APPID` | 公众号 AppID（单公众号模式） | - |
| `WECHAT_SECRET` | 公众号 AppSecret | - |
| `WECHAT_TOKEN` | 公众号 Token | - |
| `WECHAT_NAME` | 公众号名称 | - |
| `CODE_LENGTH` | 验证码长度 | 6 |
| `CODE_EXPIRE_MINUTES` | 验证码有效期（分钟） | 5 |

## 许可证

MIT License

package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/seefs001/wechat-server/config"
	"github.com/seefs001/wechat-server/wechat"
)

// ForwardResult 转发结果
type ForwardResult struct {
	ForwarderName string
	Priority      int
	Response      []byte
	StatusCode    int
	Error         error
}

// ForwardMessage 转发消息到所有配置的转发器
// 返回优先级最高的非空响应
func ForwardMessage(account *config.WechatAccount, msg *wechat.Message, rawBody []byte) []byte {
	if len(account.Forwarders) == 0 {
		return nil
	}

	// 获取消息类型/事件类型
	msgType := msg.MsgType
	eventType := ""
	if msg.Event != "" {
		eventType = strings.ToLower(msg.Event)
	}

	// 筛选需要转发的目标
	var targets []config.Forwarder
	for _, f := range account.Forwarders {
		if shouldForward(f, msgType, eventType) {
			targets = append(targets, f)
		}
	}

	if len(targets) == 0 {
		log.Printf("[%s] 没有匹配的转发目标: msgType=%s, event=%s", account.Name, msgType, eventType)
		return nil
	}

	log.Printf("[%s] 开始转发消息到 %d 个目标", account.Name, len(targets))

	// 并发转发
	var wg sync.WaitGroup
	results := make(chan ForwardResult, len(targets))

	for _, forwarder := range targets {
		wg.Add(1)
		go func(f config.Forwarder) {
			defer wg.Done()
			result := doForward(f, rawBody)
			results <- result
		}(forwarder)
	}

	// 等待所有转发完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var allResults []ForwardResult
	for r := range results {
		allResults = append(allResults, r)
		if r.Error != nil {
			log.Printf("[%s] 转发到 %s 失败: %v", account.Name, r.ForwarderName, r.Error)
		} else {
			log.Printf("[%s] 转发到 %s 成功: status=%d, responseLen=%d",
				account.Name, r.ForwarderName, r.StatusCode, len(r.Response))
		}
	}

	// 按优先级排序，选择最高优先级的有效响应
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Priority < allResults[j].Priority
	})

	for _, r := range allResults {
		if r.Error == nil && r.StatusCode == 200 && len(r.Response) > 0 {
			// 检查响应是否是有效的XML（不是简单的 "success"）
			responseStr := strings.TrimSpace(string(r.Response))
			if responseStr != "" && responseStr != "success" && strings.HasPrefix(responseStr, "<xml>") {
				log.Printf("[%s] 使用来自 %s 的响应 (优先级 %d)",
					account.Name, r.ForwarderName, r.Priority)
				return r.Response
			}
		}
	}

	return nil
}

// shouldForward 判断是否应该转发到指定转发器
func shouldForward(f config.Forwarder, msgType, eventType string) bool {
	if len(f.Events) == 0 {
		return true // 没有配置events表示全部转发
	}

	for _, e := range f.Events {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "*" || e == "all" {
			return true
		}
		if e == strings.ToLower(msgType) {
			return true
		}
		if eventType != "" && e == eventType {
			return true
		}
	}
	return false
}

// doForward 执行转发
func doForward(f config.Forwarder, body []byte) ForwardResult {
	result := ForwardResult{
		ForwarderName: f.Name,
		Priority:      f.Priority,
	}

	// 设置超时
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 5000 // 默认5秒
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	// 创建请求
	req, err := http.NewRequest("POST", f.URL, bytes.NewReader(body))
	if err != nil {
		result.Error = err
		return result
	}

	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("X-Forwarded-By", "wechat-server")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = err
		return result
	}

	result.Response = respBody
	return result
}

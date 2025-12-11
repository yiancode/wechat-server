package wechat

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Message 微信消息
type Message struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
}

// ReplyMessage 回复消息
type ReplyMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// ParseMessage 解析微信消息
func ParseMessage(data []byte) (*Message, error) {
	msg := &Message{}
	err := xml.Unmarshal(data, msg)
	if err != nil {
		return nil, fmt.Errorf("解析消息失败: %w", err)
	}
	return msg, nil
}

// NewTextReply 创建文本回复消息
func NewTextReply(toUser, fromUser, content string) *ReplyMessage {
	return &ReplyMessage{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}
}

// ToXML 将回复消息转换为 XML
func (r *ReplyMessage) ToXML() ([]byte, error) {
	return xml.Marshal(r)
}

// VerifySignature 验证微信签名
func VerifySignature(token, signature, timestamp, nonce string) bool {
	// 将 token、timestamp、nonce 按字典序排序
	arr := []string{token, timestamp, nonce}
	sort.Strings(arr)

	// 拼接成字符串并进行 SHA1 加密
	str := strings.Join(arr, "")
	hash := sha1.New()
	hash.Write([]byte(str))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	// 与 signature 比较
	return hashStr == signature
}

// GetOpenID 从消息中获取用户 OpenID
func (m *Message) GetOpenID() string {
	return m.FromUserName
}

// IsTextMessage 判断是否为文本消息
func (m *Message) IsTextMessage() bool {
	return m.MsgType == "text"
}

// IsEventMessage 判断是否为事件消息
func (m *Message) IsEventMessage() bool {
	return m.MsgType == "event"
}

// IsSubscribeEvent 判断是否为关注事件
func (m *Message) IsSubscribeEvent() bool {
	return m.MsgType == "event" && m.Event == "subscribe"
}

// IsUnsubscribeEvent 判断是否为取消关注事件
func (m *Message) IsUnsubscribeEvent() bool {
	return m.MsgType == "event" && m.Event == "unsubscribe"
}

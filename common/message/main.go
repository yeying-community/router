package message

import (
	"fmt"
	"github.com/yeying-community/router/common/config"
)

const (
	ByAll           = "all"
	ByEmail         = "email"
	ByMessagePusher = "message_pusher"
	ByDingTalk      = "dingtalk"
	ByLark          = "lark"
)

func Notify(by string, title string, description string, content string) error {
	if by == ByEmail {
		return SendEmail(title, config.RootUserEmail, content)
	}
	if by == ByMessagePusher || by == ByDingTalk || by == ByLark {
		return SendMessage(title, description, content)
	}
	return fmt.Errorf("unknown notify method: %s", by)
}

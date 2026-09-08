package notify

import (
	"go_binance_futures/models"
	"regexp"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/config"
)

// 推送格式化: #### 字段行转列表, 状态加图标, 数字去尾零
var (
	reTidyHead = regexp.MustCompile(`(?m)^#### `)
	reTidyZeroA = regexp.MustCompile(`(\.[0-9]*?)0+([^0-9]|$)`)
	reTidyZeroB = regexp.MustCompile(`\.([^0-9]|$)`)
)

func TidyMarkdown(s string) string {
	s = reTidyHead.ReplaceAllString(s, "- ")
	s = strings.ReplaceAll(s, ">失败</font>", ">❌ 失败</font>")
	s = strings.ReplaceAll(s, ">成功</font>", ">✅ 成功</font>")
	s = reTidyZeroA.ReplaceAllString(s, "$1$2")
	s = reTidyZeroB.ReplaceAllString(s, "$1")
	return s
}

func nowTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}


func getStatusColor(status string) string {
	var color string
	switch status {
		case "success":
			color = "#008000"
		case "fail":
			color = "#FF0000"
		default:
			color = "#008000"
	}
	return color
}

func GetNotifyChannel() (pusher Pusher) {
	var notification_channel, _ = config.String("notification::channel") // 通知方式
	switch (notification_channel) {
		case "dingding":
			pusher = DingDing{}
		case "slack":
			pusher = Slack{}
		default:
			pusher = DingDing{}
	}
	return pusher
}

func GetNotifyConfig(pusher Pusher) (notifyConfig models.NotifyConfig) {
	var notification_channel, _ = config.String("notification::channel") // 通知方式
	moduleName := pusher.GetModuleName()
	if moduleName == "" {
		return notifyConfig
	}
	o := orm.NewOrm()
	o.QueryTable("notify_config").
		Filter("module", moduleName).
		Filter("channel", notification_channel).
		OrderBy("-id").
		One(&notifyConfig)

	return notifyConfig
}

func IsModulePushEnabled(notifyConfig models.NotifyConfig) bool {
	return notifyConfig.ID == 0 || notifyConfig.Enable == 1
}

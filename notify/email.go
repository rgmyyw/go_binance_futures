package notify

import (
	"crypto/tls"
	"encoding/base64"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"

	"github.com/beego/beego/v2/core/config"
	"github.com/beego/beego/v2/core/logs"
)

var (
	g_email_enable, _ = config.String("email::email_enable")
	g_smtp_host, _    = config.String("email::smtp_host")
	g_smtp_port, _    = config.String("email::smtp_port")
	g_smtp_user, _    = config.String("email::smtp_user")
	g_smtp_pass, _    = config.String("email::smtp_pass")
	g_mail_to, _      = config.String("email::mail_to")
)

var reFontHTML = regexp.MustCompile(`<font color="([^"]*)">(.*?)</font>`)

// 钉钉 markdown 内联转 HTML: font 标签转彩色 span, 粗体去除
func inlineHTML(t string) string {
	t = reFontHTML.ReplaceAllString(t, `<span style="color:$1;font-weight:bold;">$2</span>`)
	return strings.ReplaceAll(t, "**", "")
}

func firstHeading(s string) string {
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") {
			return strings.TrimSpace(strings.TrimLeft(t, "# "))
		}
	}
	return ""
}

// markdown 转 HTML 卡片邮件
func mdToHTML(s string) string {
	var body string
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			h := strings.TrimSpace(strings.TrimLeft(t, "#"))
			body += `<div style="padding:12px 18px 10px;font-size:17px;font-weight:bold;color:#111;">` + inlineHTML(h) + `</div>`
			continue
		}
		t = strings.TrimPrefix(t, "- ")
		t = inlineHTML(t)
		parts := strings.SplitN(t, "：", 2)
		if len(parts) == 2 {
			body += `<tr><td style="padding:7px 18px;color:#8a8a8a;font-size:13px;white-space:nowrap;vertical-align:top;">` + parts[0] + `</td><td style="padding:7px 18px;font-size:14px;color:#222;">` + parts[1] + `</td></tr>`
		} else {
			body += `<div style="padding:7px 18px;font-size:14px;color:#222;">` + t + `</div>`
		}
	}
	return `<!DOCTYPE html><html><body style="margin:0;padding:0;background-color:#f4f5f7;">
<div style="max-width:420px;margin:20px auto;font-family:-apple-system,'PingFang SC','Microsoft YaHei',sans-serif;background-color:#ffffff;border:1px solid #e5e5e5;border-radius:12px;overflow:hidden;">
<div style="background-color:#16213e;padding:14px 18px;"><span style="color:#ffffff;font-size:15px;font-weight:bold;">📈 币安合约交易机器人</span></div>
` + body + `
<div style="padding:10px 18px 14px;color:#c0c0c0;font-size:11px;border-top:1px solid #f0f0f0;">go_binance_futures · VM101 自动发送</div>
</div></body></html>`
}

func b64Subject(s string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

// SendEmail 与钉钉并行的邮件通道(QQ SMTP SSL 465), HTML 卡片, 本地补丁
func SendEmail(content string) {
	if g_email_enable != "1" || g_smtp_host == "" || g_smtp_user == "" || g_mail_to == "" {
		return
	}
	go func() {
		defer func() {
			if e := recover(); e != nil {
				logs.Error("email send panic:", e)
			}
		}()
		port, err := strconv.Atoi(g_smtp_port)
		if err != nil || port == 0 {
			port = 465
		}
		host := g_smtp_host
		addr := host + ":" + strconv.Itoa(port)
		from := b64Subject("币安交易机器人") + " <" + g_smtp_user + ">"
		subject := firstHeading(content)
		if subject == "" {
			subject = "币安机器人通知"
		}
		msg := strings.Join([]string{
			"From: " + from,
			"To: " + g_mail_to,
			"Subject: " + b64Subject(subject),
			"MIME-Version: 1.0",
			"Content-Type: text/html; charset=UTF-8",
			"",
			mdToHTML(content),
		}, "\r\n")
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			logs.Error("email tls dial:", err)
			return
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			logs.Error("email client:", err)
			conn.Close()
			return
		}
		defer c.Close()
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(smtp.PlainAuth("", g_smtp_user, g_smtp_pass, host)); err != nil {
				logs.Error("email auth:", err)
				return
			}
		}
		if err = c.Mail(g_smtp_user); err != nil {
			logs.Error("email mail from:", err)
			return
		}
		if err = c.Rcpt(g_mail_to); err != nil {
			logs.Error("email rcpt:", err)
			return
		}
		w, err := c.Data()
		if err != nil {
			logs.Error("email data:", err)
			return
		}
		if _, err = w.Write([]byte(msg)); err != nil {
			logs.Error("email write:", err)
			return
		}
		w.Close()
		if err = c.Quit(); err != nil {
			logs.Error("email quit:", err)
			return
		}
		logs.Info("email sent to " + g_mail_to + " [" + subject + "]")
	}()
}

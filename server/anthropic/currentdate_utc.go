package anthropic

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/realityone/cocoq/server/proxy"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var currentDateLinePattern = regexp.MustCompile(`Today's date is (\d{4}-\d{2}-\d{2})\.`)

type currentDateUTC struct {
	now func() time.Time
}

func (c currentDateUTC) Handle(ctx *proxy.OnContext[proxy.ReqCtx]) {
	req := ctx.Opaque.Request
	if req.Body == nil {
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read anthropic request body for currentDate rewrite")
		return
	}
	defer req.Body.Close()

	utcDate := c.utcDate()
	nextBody, changed, err := c.forcePromptCurrentDateUTC(body, utcDate)
	if err != nil {
		logrus.WithError(err).Warn("failed to rewrite anthropic request body currentDate")
		nextBody = body
	}
	if changed {
		logrus.WithField("utc_date", utcDate).Debug("forced anthropic currentDate prompt block to UTC date")
	}

	proxy.ReplaceRequestBody(req, nextBody)
	ctx.Opaque.PostRequest = req
}

func (c currentDateUTC) forcePromptCurrentDateUTC(body []byte, utcDate string) ([]byte, bool, error) {
	if !gjson.ValidBytes(body) {
		return body, false, nil
	}

	nextBody := body
	changed := false

	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body, false, nil
	}
	for msgIdx, message := range messages.Array() {
		content := message.Get("content")
		if !content.IsArray() {
			continue
		}
		for cntIdx, block := range content.Array() {
			if block.Get("type").String() != "text" {
				continue
			}
			nextText, ok := c.rewriteCurrentDateText(block.Get("text").String(), utcDate)
			if !ok {
				continue
			}
			var err error
			nextBody, err = sjson.SetBytes(nextBody, fmt.Sprintf("messages.%d.content.%d.text", msgIdx, cntIdx), nextText)
			if err != nil {
				logrus.WithError(err).
					WithFields(logrus.Fields{
						"message_index": msgIdx,
						"content_index": cntIdx,
					}).
					Warn("failed to force anthropic currentDate prompt block to UTC date")
				continue
			}
			changed = true
		}
	}

	return nextBody, changed, nil
}

func (c currentDateUTC) rewriteCurrentDateText(text string, utcDate string) (string, bool) {
	if !strings.Contains(text, "# currentDate") || !strings.Contains(text, "Today's date is ") {
		return text, false
	}

	idx := currentDateLinePattern.FindStringSubmatchIndex(text)
	if idx == nil {
		return text, false
	}

	dateStart := idx[2]
	dateEnd := idx[3]
	if text[dateStart:dateEnd] == utcDate {
		return text, false
	}

	return text[:dateStart] + utcDate + text[dateEnd:], true
}

func (c currentDateUTC) utcDate() string {
	return c.localNow().UTC().Format(time.DateOnly)
}

func (c currentDateUTC) localNow() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

package anthropic

import (
	"fmt"
	"io"

	"cocoq/server/proxy"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const cacheControlTTL1h = "1h"

type caching1h struct{}

func (c caching1h) Handle(ctx *proxy.OnContext[proxy.ReqCtx]) {
	req := ctx.Opaque.Request
	if req.Body == nil {
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logrus.WithError(err).Warn("failed to read anthropic request body for cache ttl rewrite")
		return
	}
	defer req.Body.Close()

	nextBody, changed, err := c.forcePromptCacheControlTTL1h(body)
	if err != nil {
		logrus.WithError(err).Warn("failed to rewrite anthropic request body cache ttl")
		nextBody = body
	}
	if changed {
		logrus.Debug("forced anthropic prompt cache_control ttl to 1h")
	}

	proxy.ReplaceRequestBody(req, nextBody)
	ctx.Opaque.PostRequest = req
}

func (c caching1h) forcePromptCacheControlTTL1h(body []byte) ([]byte, bool, error) {
	if !gjson.ValidBytes(body) {
		return body, false, nil
	}

	nextBody := body
	changed := false

	setTTL := func(path string) error {
		if gjson.GetBytes(nextBody, path).String() == cacheControlTTL1h {
			return nil
		}
		_nextBody, err := sjson.SetBytes(nextBody, path, cacheControlTTL1h)
		if err != nil {
			return err
		}
		changed = true
		nextBody = _nextBody
		return nil
	}

	func() {
		system := gjson.GetBytes(body, "system")
		if !system.IsArray() {
			return
		}
		for idx, block := range system.Array() {
			if c.hasCacheControl(block) {
				if err := setTTL(fmt.Sprintf("system.%d.cache_control.ttl", idx)); err != nil {
					logrus.WithError(err).
						WithField("system_index", idx).
						Warn("failed to force anthropic system cache_control ttl to 1h")
					continue
				}
			}
		}
	}()

	func() {
		messages := gjson.GetBytes(body, "messages")
		if !messages.IsArray() {
			return
		}
		for msgIdx, message := range messages.Array() {
			content := message.Get("content")
			if !content.IsArray() {
				continue
			}
			for cntIdx, block := range content.Array() {
				if c.hasCacheControl(block) {
					if err := setTTL(fmt.Sprintf("messages.%d.content.%d.cache_control.ttl", msgIdx, cntIdx)); err != nil {
						logrus.WithError(err).
							WithFields(logrus.Fields{
								"message_index": msgIdx,
								"content_index": cntIdx,
							}).
							Warn("failed to force anthropic message cache_control ttl to 1h")
						continue
					}
				}
			}
		}
	}()

	return nextBody, changed, nil
}

func (c caching1h) hasCacheControl(block gjson.Result) bool {
	return block.Get("cache_control.type").String() == "ephemeral"
}

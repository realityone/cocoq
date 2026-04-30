package anthropic

import (
	"fmt"

	"cocoq/server/proxy"
)

type example struct{}

func (e *example) HandleRequest(*proxy.OnContext[proxy.ReqCtx]) {
	fmt.Println("HandleRequest in example")
}

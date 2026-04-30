package server

import (
	"net/http"
	"reflect"
	"testing"
)

func TestOnRequestContextNext(t *testing.T) {
	req := &http.Request{Method: http.MethodPost}
	ctx := &OnRequestContext{
		Tuple: OnRequestTuple{
			Request: req,
		},
		index: -1,
	}

	var calls []string
	ctx.handlers = RequestHandlersChain{
		func(c *OnRequestContext) {
			if c.Tuple.Request != req {
				t.Fatal("handler received unexpected request context")
			}
			calls = append(calls, "first")
		},
		nil,
		func(c *OnRequestContext) {
			calls = append(calls, "second")
		},
	}

	ctx.Next()

	if want := []string{"first", "second"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if ctx.index != int8(len(ctx.handlers)) {
		t.Fatalf("index = %d, want %d", ctx.index, len(ctx.handlers))
	}
}

func TestOnResponseContextNext(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusAccepted}
	ctx := &OnResponseContext{
		Tuple: OnResponseTuple{
			Response: resp,
		},
		index: -1,
	}

	var calls []string
	ctx.handlers = ResponseHandlersChain{
		func(c *OnResponseContext) {
			if c.Tuple.Response != resp {
				t.Fatal("handler received unexpected response context")
			}
			calls = append(calls, "first")
		},
		nil,
		func(c *OnResponseContext) {
			calls = append(calls, "second")
		},
	}

	ctx.Next()

	if want := []string{"first", "second"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if ctx.index != int8(len(ctx.handlers)) {
		t.Fatalf("index = %d, want %d", ctx.index, len(ctx.handlers))
	}
}

func TestContextIsAborted(t *testing.T) {
	requestCtx := &OnRequestContext{index: abortIndex}
	if !requestCtx.IsAborted() {
		t.Fatal("request context should be aborted")
	}

	responseCtx := &OnResponseContext{index: abortIndex}
	if !responseCtx.IsAborted() {
		t.Fatal("response context should be aborted")
	}
}

func TestHandleOnRequestDefaultsToOriginalRequest(t *testing.T) {
	req := &http.Request{Method: http.MethodPost}

	gotReq, gotResp := handleOnRequest(req, nil)

	if gotReq != req {
		t.Fatal("handleOnRequest should return original request when no handler overrides it")
	}
	if gotResp != nil {
		t.Fatalf("response = %v, want nil", gotResp)
	}
}

func TestHandleOnRequestUsesHandlerResult(t *testing.T) {
	req := &http.Request{Method: http.MethodPost}
	replacementReq := &http.Request{Method: http.MethodGet}
	response := &http.Response{StatusCode: http.StatusTeapot}

	gotReq, gotResp := handleOnRequest(req, nil, func(c *OnRequestContext) {
		if c.Tuple.Request != req {
			t.Fatal("handler received unexpected request")
		}
		c.Tuple.PostRequest = replacementReq
		c.Tuple.PostResponse = response
	})

	if gotReq != replacementReq {
		t.Fatal("handleOnRequest should return handler request override")
	}
	if gotResp != response {
		t.Fatal("handleOnRequest should return handler response override")
	}
}

func TestHandleOnResponseDefaultsToOriginalResponse(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusAccepted}

	gotResp := handleOnResponse(resp, nil)

	if gotResp != resp {
		t.Fatal("handleOnResponse should return original response when no handler overrides it")
	}
}

func TestHandleOnResponseUsesHandlerResult(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusAccepted}
	replacementResp := &http.Response{StatusCode: http.StatusNoContent}

	gotResp := handleOnResponse(resp, nil, func(c *OnResponseContext) {
		if c.Tuple.Response != resp {
			t.Fatal("handler received unexpected response")
		}
		c.Tuple.PostResponse = replacementResp
	})

	if gotResp != replacementResp {
		t.Fatal("handleOnResponse should return handler response override")
	}
}

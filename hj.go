// Package hj contains HTTP+JSON tools.
//
// The code in this package is liberally adapted in part
// from github.com/chain/chain.
package hj

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"reflect"
)

type jsonHandler struct {
	fval                reflect.Value
	argType, resultType reflect.Type
	hasCtx, hasErr      bool
	onError             func(context.Context, error)
}

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

// Handler produces an http.Handler from a wrapped function f.
//
// The function may take one "request" argument of any type that can be JSON-unmarshaled.
// That argument can optionally be preceded by a context.Context.
// It may return one "response" value of any type that can be JSON-marshaled.
// That return value can optionally be followed by an error.
// If the error implements Responder,
// its Respond method is used to set the status code for the HTTP response;
// otherwise the status code on error is 500 (internal server error).
//
// When the handler is invoked,
// the request is checked to ensure that the method is POST
// and the Content-Type is application/json.
// Then the function f is invoked with the request body JSON-unmarshaled into its argument
// (if there is one).
// The return value of f (if there is one) is JSON-marshaled into the response
// and the Content-Type of the response is set to application/json.
//
// If f takes a context.Context, it receives the context object from the http.Request.
// If f returns an error, it is passed to the optional onError callback, along with the context.
// The context object in both cases is adorned with the pending *http.Request,
// which can be retrieved with the Request function,
// and the pending http.ResponseWriter,
// which can be retrieved with Response.
func Handler(f interface{}, onError func(context.Context, error)) http.Handler {
	var (
		fval  = reflect.ValueOf(f)
		ftype = fval.Type()
	)
	if ftype.Kind() != reflect.Func {
		panic("non-function passed to hj.Handler")
	}
	if ftype.IsVariadic() {
		panic("variadic function passed to hj.Handler")
	}

	var (
		hasCtx  bool
		argType reflect.Type
	)

	switch ftype.NumIn() {
	case 0:
		// do nothing
	case 1:
		if ftype.In(0).Implements(contextType) {
			hasCtx = true
		} else {
			argType = ftype.In(0)
		}
	case 2:
		if ftype.In(0).Implements(contextType) {
			hasCtx = true
		} else {
			panic("two-arg function passed to hj.Handler with non-context first arg")
		}
		argType = ftype.In(1)
	default:
		panic(fmt.Sprintf("%d-ary function passed to hj.Handler", ftype.NumIn()))
	}

	var (
		hasErr     bool
		resultType reflect.Type
	)

	switch ftype.NumOut() {
	case 0:
		// do nothing
	case 1:
		if ftype.Out(0).Implements(errorType) {
			hasErr = true
		} else {
			resultType = ftype.Out(0)
		}
	case 2:
		if ftype.Out(1).Implements(errorType) {
			hasErr = true
		} else {
			panic("two-valued function passed to hj.Handler with non-error second value")
		}
		resultType = ftype.Out(0)
	default:
		panic(fmt.Sprintf("%d-valued function passed to hj.Handler", ftype.NumOut()))
	}

	return jsonHandler{
		fval:       fval,
		hasCtx:     hasCtx,
		hasErr:     hasErr,
		argType:    argType,
		resultType: resultType,
		onError:    onError,
	}
}

// ServeHTTP implements http.Handler.
func (h jsonHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	ctx = context.WithValue(ctx, reqKey{}, req)
	ctx = context.WithValue(ctx, respKey{}, w)

	handleErr := func(err error) {
		if r, ok := err.(Responder); ok {
			r.Respond(w)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		if h.onError != nil {
			h.onError(ctx, err)
		}
	}

	if req.Method != "POST" {
		handleErr(ErrNotPost{Method: req.Method})
		return
	}
	ct, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || ct != "application/json" {
		handleErr(ErrNotJSON{ContentType: ct})
		return
	}

	var args []reflect.Value

	if h.hasCtx {
		args = append(args, reflect.ValueOf(ctx))
	}
	if h.argType != nil {
		argPtr := reflect.New(h.argType)

		dec := json.NewDecoder(req.Body)
		dec.UseNumber()
		err := dec.Decode(argPtr.Interface())
		if err != nil {
			handleErr(ErrDecode{Err: err})
			return
		}
		args = append(args, argPtr.Elem())
	}

	rv := h.fval.Call(args)

	if h.hasErr {
		err, _ := rv[len(rv)-1].Interface().(error)
		if err != nil {
			handleErr(err)
			return
		}
	}

	if h.resultType == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	res := rv[0].Interface()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	err = enc.Encode(res)
	if err != nil {
		handleErr(ErrEncode{Err: err})
		return
	}
}

type (
	reqKey  struct{}
	respKey struct{}
)

// Request returns the pending HTTP request object
// from the context optionally passed to the wrapped function
// (which is also passed to the optional onError callback).
func Request(ctx context.Context) *http.Request {
	val := ctx.Value(reqKey{})
	if val == nil {
		return nil
	}
	return val.(*http.Request)
}

// Response returns the pending HTTP ResponseWriter object
// from the context optionally passed to the wrapped function
// (which is also passed to the optional onError callback).
func Response(ctx context.Context) http.ResponseWriter {
	val := ctx.Value(respKey{})
	if val == nil {
		return nil
	}
	return val.(http.ResponseWriter)
}

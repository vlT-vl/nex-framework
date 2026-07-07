// Package core contains shared framework types: Context, HandlerFunc, Host,
// Registrar, RPCError. Kept separate to avoid circular imports.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"nex/internal/session"
)

type HandlerFunc func(c *Context, params json.RawMessage) (any, error)

// SecurityDecision describes a privileged or user-mediated operation before it
// is executed. Applications can inspect it through Config.SecurityPolicy or the
// focused hooks in Config. Nil policy/hooks mean allow.
type SecurityDecision struct {
	Method    string          `json:"method"`
	Category  string          `json:"category"`
	Operation string          `json:"operation"`
	Path      string          `json:"path,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	URL       string          `json:"url,omitempty"`
	Command   string          `json:"command,omitempty"`
	JS        string          `json:"js,omitempty"`
	Recursive bool            `json:"recursive,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
}

// SecurityPolicy is optional. Returning nil allows the operation; returning an
// error denies it and the frontend receives an RPC error.
type SecurityPolicy interface {
	Allow(*Context, SecurityDecision) error
}

// Host is what the App exposes to handlers (main-thread dispatch, events, window).
type Host interface {
	Authorize(*Context, SecurityDecision) error
	OnMain(fn func())
	Emit(event string, payload any)
	Eval(js string)
	SetTitle(title string)
	SetSize(w, h int, hint string)
	WindowInfo() map[string]any
	Quit()
	// ReleaseSingleInstance closes the single-instance lock (if any) so a
	// freshly spawned process of the same app can acquire it immediately.
	// Safe to call more than once. Must be called before starting a
	// replacement process (e.g. for app.restart), otherwise the new process
	// can race the old one for the lock and treat it as a second instance.
	ReleaseSingleInstance()
}

// Registrar lets sub-packages register handlers without importing App.
type Registrar interface {
	Register(method string, fn HandlerFunc)
}

type Context struct {
	Ctx     context.Context
	Host    Host
	Session *session.Session
	Request *http.Request
	Method  string
}

func (c *Context) Authorize(d SecurityDecision) error {
	if d.Method == "" {
		d.Method = c.Method
	}
	if err := c.Host.Authorize(c, d); err != nil {
		if rpcErr, ok := err.(*RPCError); ok {
			return rpcErr
		}
		return Errorf("forbidden", "%v", err)
	}
	return nil
}

func (c *Context) OnMain(fn func())              { c.Host.OnMain(fn) }
func (c *Context) Emit(event string, p any)      { c.Host.Emit(event, p) }
func (c *Context) Eval(js string)                { c.Host.Eval(js) }
func (c *Context) SetTitle(t string)             { c.Host.SetTitle(t) }
func (c *Context) SetSize(w, h int, hint string) { c.Host.SetSize(w, h, hint) }
func (c *Context) WindowInfo() map[string]any    { return c.Host.WindowInfo() }
func (c *Context) Quit()                         { c.Host.Quit() }
func (c *Context) ReleaseSingleInstance()        { c.Host.ReleaseSingleInstance() }

func (c *Context) Bind(params json.RawMessage, v any) error {
	if len(params) == 0 {
		return nil
	}
	return json.Unmarshal(params, v)
}

type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

func Errorf(code, format string, a ...any) *RPCError {
	return &RPCError{Code: code, Message: fmt.Sprintf(format, a...)}
}

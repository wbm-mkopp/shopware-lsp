package lsp

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
)

const requestQueueCapacity = 128
const requestCancelledCode = -32800

type queuedRequest struct {
	ctx     context.Context
	conn    *jsonrpc2.Conn
	request *jsonrpc2.Request
	cancel  context.CancelFunc
}

// requestDispatcher keeps document notifications and normal feature requests
// ordered on one worker. The transport reader remains free to cancel running
// or queued work without enabling concurrent mutation of server state.
type requestDispatcher struct {
	server   *Server
	handler  jsonrpc2.Handler
	ctx      context.Context
	cancel   context.CancelFunc
	queue    chan queuedRequest
	done     chan struct{}
	mu       sync.Mutex
	requests map[jsonrpc2.ID]context.CancelFunc
}

func newRequestDispatcher(handler jsonrpc2.Handler) *requestDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := &requestDispatcher{handler: handler, ctx: ctx, cancel: cancel, queue: make(chan queuedRequest, requestQueueCapacity), done: make(chan struct{}), requests: make(map[jsonrpc2.ID]context.CancelFunc)}
	go dispatcher.run()
	return dispatcher
}
func (d *requestDispatcher) Handle(_ context.Context, conn *jsonrpc2.Conn, request *jsonrpc2.Request) {
	if request.Method == "$/cancelRequest" {
		var params struct {
			ID jsonrpc2.ID `json:"id"`
		}
		if request.Params != nil && json.Unmarshal(*request.Params, &params) == nil {
			d.mu.Lock()
			cancel := d.requests[params.ID]
			d.mu.Unlock()
			if cancel != nil {
				cancel()
			}
		}
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	if !request.Notif {
		d.mu.Lock()
		d.requests[request.ID] = cancel
		d.mu.Unlock()
	}
	work := queuedRequest{ctx: ctx, conn: conn, request: request, cancel: cancel}
	select {
	case d.queue <- work:
	case <-d.ctx.Done():
		cancel()
	}
}
func (d *requestDispatcher) run() {
	defer close(d.done)
	for {
		select {
		case <-d.ctx.Done():
			return
		case work := <-d.queue:
			if d.server != nil && work.request.Method == "textDocument/diagnostic" && d.server.initializationOptions.CLIMode {
				if d.server.startBackground(func(ctx context.Context) {
					stop := context.AfterFunc(ctx, work.cancel)
					defer stop()
					d.execute(work)
				}) {
					continue
				}
			}
			d.execute(work)
		}
	}
}
func (d *requestDispatcher) execute(work queuedRequest) {
	defer work.cancel()
	defer func() {
		if !work.request.Notif {
			d.mu.Lock()
			delete(d.requests, work.request.ID)
			d.mu.Unlock()
		}
	}()
	if work.ctx.Err() != nil && !work.request.Notif {
		_ = work.conn.ReplyWithError(d.ctx, work.request.ID, &jsonrpc2.Error{Code: requestCancelledCode, Message: "request cancelled"})
	} else {
		d.handler.Handle(work.ctx, work.conn, work.request)
	}
}
func (d *requestDispatcher) close() { d.cancel(); <-d.done }

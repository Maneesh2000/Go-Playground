package agent

import (
	"encoding/base64"
	"log/slog"

	"github.com/gorilla/websocket"
)

// Msg is the single wire format for both directions over the agent WebSocket.
// Terminal payloads are base64 so binary bytes survive JSON text frames.
type Msg struct {
	Ch  string `json:"ch"`            // "fs" | "term"
	Op  string `json:"op"`            // operation or result kind
	ID  int64  `json:"id,omitempty"`  // request id (fs request/response correlation)
	Err string `json:"err,omitempty"` // non-empty => operation failed

	// fs fields
	Path       string     `json:"path,omitempty"`
	To         string     `json:"to,omitempty"`
	ContentB64 string     `json:"content_b64,omitempty"`
	Entries    []DirEntry `json:"entries,omitempty"`

	// term fields
	Cols    uint16 `json:"cols,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
	DataB64 string `json:"data_b64,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// Session multiplexes fs + term services over one WebSocket connection.
type Session struct {
	ws   *websocket.Conn
	send chan Msg
	fs   *FS
	term *Term
}

// Serve runs the agent protocol on ws until the connection closes. root is the
// workspace directory (e.g. /workspace). It blocks.
func Serve(ws *websocket.Conn, root string) {
	s := &Session{
		ws:   ws,
		send: make(chan Msg, 256),
		fs:   NewFS(root),
		term: NewTerm(root),
	}
	done := make(chan struct{})
	go s.writeLoop(done)
	s.readLoop()
	s.term.Close()
	close(done)
}

// writeLoop is the sole writer to the WebSocket (gorilla requires serialized
// writes); everything else enqueues onto s.send.
func (s *Session) writeLoop(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case m := <-s.send:
			if err := s.ws.WriteJSON(m); err != nil {
				return
			}
		}
	}
}

func (s *Session) emit(m Msg) {
	select {
	case s.send <- m:
	default: // drop if the client can't keep up rather than block the reader
		slog.Warn("agent send buffer full, dropping message", "ch", m.Ch, "op", m.Op)
	}
}

func (s *Session) readLoop() {
	for {
		var m Msg
		if err := s.ws.ReadJSON(&m); err != nil {
			return
		}
		switch m.Ch {
		case "fs":
			s.handleFS(m)
		case "term":
			s.handleTerm(m)
		default:
			s.emit(Msg{Ch: m.Ch, Op: "result", ID: m.ID, Err: "unknown channel"})
		}
	}
}

// --- fs dispatch ---

func (s *Session) handleFS(m Msg) {
	res := Msg{Ch: "fs", Op: "result", ID: m.ID}
	switch m.Op {
	case "list":
		entries, err := s.fs.List(m.Path)
		if err != nil {
			res.Err = err.Error()
		} else {
			res.Entries = entries
			res.Path = m.Path
		}
	case "read":
		data, err := s.fs.Read(m.Path)
		if err != nil {
			res.Err = err.Error()
		} else {
			res.ContentB64 = base64.StdEncoding.EncodeToString(data)
			res.Path = m.Path
		}
	case "write":
		data, err := base64.StdEncoding.DecodeString(m.ContentB64)
		if err != nil {
			res.Err = "invalid content_b64"
		} else if err := s.fs.Write(m.Path, data); err != nil {
			res.Err = err.Error()
		}
	case "mkdir":
		if err := s.fs.Mkdir(m.Path); err != nil {
			res.Err = err.Error()
		}
	case "delete":
		if err := s.fs.Delete(m.Path); err != nil {
			res.Err = err.Error()
		}
	case "rename":
		if err := s.fs.Rename(m.Path, m.To); err != nil {
			res.Err = err.Error()
		}
	default:
		res.Err = "unknown fs op: " + m.Op
	}
	s.emit(res)
}

// --- term dispatch ---

func (s *Session) handleTerm(m Msg) {
	switch m.Op {
	case "start":
		cols, rows := m.Cols, m.Rows
		if cols == 0 {
			cols = 80
		}
		if rows == 0 {
			rows = 24
		}
		err := s.term.Start(cols, rows,
			func(b []byte) {
				s.emit(Msg{Ch: "term", Op: "data", DataB64: base64.StdEncoding.EncodeToString(b)})
			},
			func(code int) {
				s.emit(Msg{Ch: "term", Op: "exit", Code: code})
			},
		)
		if err != nil {
			s.emit(Msg{Ch: "term", Op: "exit", Code: 1, Err: err.Error()})
		}
	case "stdin":
		if data, err := base64.StdEncoding.DecodeString(m.DataB64); err == nil {
			s.term.Write(data)
		}
	case "resize":
		s.term.Resize(m.Cols, m.Rows)
	}
}

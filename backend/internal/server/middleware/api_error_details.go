package middleware

import (
	"bytes"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	apiErrorDetailsHint       = " (View error details at https://tasksetly.com.)"
	maxAPIErrorResponseBuffer = 1 << 20
)

type apiErrorResponseMode uint8

const (
	apiErrorResponseUnknown apiErrorResponseMode = iota
	apiErrorResponsePassthrough
	apiErrorResponseJSON
	apiErrorResponseSSE
)

// APIGatewayErrorDetails adds a support link to gateway error messages while
// preserving the response protocol and status code.
func APIGatewayErrorDetails() gin.HandlerFunc {
	return func(c *gin.Context) {
		writer := &apiErrorDetailsWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		defer func() {
			c.Writer = writer.ResponseWriter
			writer.finish()
		}()
		c.Next()
	}
}

type apiErrorDetailsWriter struct {
	gin.ResponseWriter
	mode       apiErrorResponseMode
	jsonBody   bytes.Buffer
	ssePending []byte
	buffered   bool
}

func (w *apiErrorDetailsWriter) WriteHeader(code int) {
	if w.Written() {
		return
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *apiErrorDetailsWriter) WriteHeaderNow() {
	w.selectMode()
	if w.mode == apiErrorResponseJSON {
		w.buffered = true
		return
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *apiErrorDetailsWriter) Write(p []byte) (int, error) {
	w.selectMode()
	switch w.mode {
	case apiErrorResponseJSON:
		if w.jsonBody.Len()+len(p) > maxAPIErrorResponseBuffer {
			if err := w.flushJSONBody(); err != nil {
				return 0, err
			}
			w.mode = apiErrorResponsePassthrough
			return w.ResponseWriter.Write(p)
		}
		w.buffered = true
		return w.jsonBody.Write(p)
	case apiErrorResponseSSE:
		w.buffered = true
		if err := w.writeSSE(p, false); err != nil {
			return 0, err
		}
		return len(p), nil
	default:
		return w.ResponseWriter.Write(p)
	}
}

func (w *apiErrorDetailsWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *apiErrorDetailsWriter) Status() int {
	return w.ResponseWriter.Status()
}

func (w *apiErrorDetailsWriter) Size() int {
	if w.mode == apiErrorResponseJSON && w.buffered {
		return w.jsonBody.Len()
	}
	if w.mode == apiErrorResponseSSE && w.buffered {
		size := w.ResponseWriter.Size()
		if size < 0 {
			return len(w.ssePending)
		}
		return size + len(w.ssePending)
	}
	return w.ResponseWriter.Size()
}

func (w *apiErrorDetailsWriter) Written() bool {
	return w.buffered || w.ResponseWriter.Written()
}

func (w *apiErrorDetailsWriter) Flush() {
	w.selectMode()
	if w.mode == apiErrorResponseJSON {
		return
	}
	w.ResponseWriter.WriteHeaderNow()
	w.ResponseWriter.Flush()
}

func (w *apiErrorDetailsWriter) selectMode() {
	if w.mode != apiErrorResponseUnknown {
		return
	}
	mediaType, _, _ := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if w.Status() >= http.StatusBadRequest && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")) {
		w.mode = apiErrorResponseJSON
		return
	}
	if mediaType == "text/event-stream" {
		w.mode = apiErrorResponseSSE
		return
	}
	w.mode = apiErrorResponsePassthrough
}

func (w *apiErrorDetailsWriter) finish() {
	w.selectMode()
	switch w.mode {
	case apiErrorResponseJSON:
		body := addAPIErrorDetailsToJSON(w.jsonBody.Bytes())
		if len(body) != w.jsonBody.Len() {
			w.Header().Del("Content-Length")
		}
		w.ResponseWriter.WriteHeaderNow()
		_, _ = w.ResponseWriter.Write(body)
	case apiErrorResponseSSE:
		_ = w.writeSSE(nil, true)
	}
}

func (w *apiErrorDetailsWriter) flushJSONBody() error {
	w.ResponseWriter.WriteHeaderNow()
	_, err := w.ResponseWriter.Write(w.jsonBody.Bytes())
	w.jsonBody.Reset()
	w.buffered = false
	return err
}

func (w *apiErrorDetailsWriter) writeSSE(p []byte, final bool) error {
	w.ssePending = append(w.ssePending, p...)
	for {
		start, end := sseFrameEnd(w.ssePending)
		if start < 0 {
			break
		}
		if !w.ResponseWriter.Written() {
			w.ResponseWriter.WriteHeaderNow()
		}
		frame := w.ssePending[:end]
		if len(frame) <= maxAPIErrorResponseBuffer {
			frame = transformAPIErrorSSEFrame(frame)
		}
		w.ssePending = w.ssePending[end:]
		if _, err := w.ResponseWriter.Write(frame); err != nil {
			return err
		}
	}
	if len(w.ssePending) > maxAPIErrorResponseBuffer {
		if !w.ResponseWriter.Written() {
			w.ResponseWriter.WriteHeaderNow()
		}
		_, err := w.ResponseWriter.Write(w.ssePending)
		w.ssePending = nil
		w.mode = apiErrorResponsePassthrough
		return err
	}
	if final && len(w.ssePending) != 0 {
		if !w.ResponseWriter.Written() {
			w.ResponseWriter.WriteHeaderNow()
		}
		_, err := w.ResponseWriter.Write(transformAPIErrorSSEFrame(w.ssePending))
		w.ssePending = nil
		return err
	}
	return nil
}

func sseFrameEnd(data []byte) (start, end int) {
	lf := bytes.Index(data, []byte("\n\n"))
	crlf := bytes.Index(data, []byte("\r\n\r\n"))
	switch {
	case lf < 0 && crlf < 0:
		return -1, -1
	case crlf >= 0 && (lf < 0 || crlf < lf):
		return crlf, crlf + 4
	default:
		return lf, lf + 2
	}
}

func transformAPIErrorSSEFrame(frame []byte) []byte {
	lines := bytes.Split(frame, []byte("\n"))
	var eventType string
	dataLine := -1
	var payload []byte
	for i, line := range lines {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("event:"))))
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			if dataLine >= 0 {
				return frame
			}
			dataLine = i
			payload = bytes.TrimPrefix(line, []byte("data:"))
			payload = bytes.TrimPrefix(payload, []byte(" "))
		}
	}
	if dataLine < 0 {
		return frame
	}
	if eventType != "error" && eventType != "response.failed" &&
		!gjson.GetBytes(payload, "error").Exists() && !gjson.GetBytes(payload, "response.error").Exists() {
		return frame
	}
	updated, changed := appendAPIErrorDetails(payload, []string{"error.message", "response.error.message", "message", "detail", "error"})
	if !changed {
		return frame
	}

	line := lines[dataLine]
	contentEnd := len(line)
	if contentEnd > 0 && line[contentEnd-1] == '\r' {
		contentEnd--
	}
	prefixEnd := bytes.Index(line[:contentEnd], []byte("data:")) + len("data:")
	if prefixEnd < contentEnd && line[prefixEnd] == ' ' {
		prefixEnd++
	}
	newLine := make([]byte, 0, prefixEnd+len(updated)+len(line)-contentEnd)
	newLine = append(newLine, line[:prefixEnd]...)
	newLine = append(newLine, updated...)
	newLine = append(newLine, line[contentEnd:]...)
	lines[dataLine] = newLine
	return bytes.Join(lines, []byte("\n"))
}

func addAPIErrorDetailsToJSON(body []byte) []byte {
	updated, changed := appendAPIErrorDetails(body, []string{"error.message", "response.error.message", "message", "detail", "error"})
	if !changed {
		return body
	}
	return updated
}

func appendAPIErrorDetails(body []byte, paths []string) ([]byte, bool) {
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		message := strings.TrimSpace(value.String())
		if message == "" || strings.Contains(strings.ToLower(message), "tasksetly.com") {
			return body, false
		}
		updated, err := sjson.SetBytes(body, path, message+apiErrorDetailsHint)
		if err != nil {
			return body, false
		}
		return updated, true
	}
	return body, false
}

var _ gin.ResponseWriter = (*apiErrorDetailsWriter)(nil)

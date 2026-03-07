package psb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// waasRoundTripper sends WaaS requests as raw HTTP/1.0 and reads the response
// manually to tolerate duplicate Transfer-Encoding: chunked from 9PSB/proxy.
type waasRoundTripper struct {
	host    string
	dialer  *net.Dialer
	timeout time.Duration
}

func newWaasRoundTripper(host string, timeout time.Duration) *waasRoundTripper {
	return &waasRoundTripper{
		host:    host,
		dialer:  &net.Dialer{Timeout: 15 * time.Second},
		timeout: timeout,
	}
}

func (rt *waasRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != rt.host {
		return nil, fmt.Errorf("waas round tripper: host mismatch %q", req.URL.Host)
	}
	ctx := req.Context()
	conn, err := rt.dialer.DialContext(ctx, "tcp", rt.host)
	if err != nil {
		return nil, fmt.Errorf("waas dial: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	reqLine := fmt.Sprintf("%s %s HTTP/1.0\r\n", req.Method, req.URL.RequestURI())
	buf := bytes.NewBufferString(reqLine)
	buf.WriteString("Host: " + req.URL.Host + "\r\n")
	buf.WriteString("Content-Type: " + contentType + "\r\n")
	buf.WriteString("Content-Length: " + strconv.Itoa(len(body)) + "\r\n")
	if auth := req.Header.Get("Authorization"); auth != "" {
		buf.WriteString("Authorization: " + auth + "\r\n")
	}
	buf.WriteString("Connection: close\r\n")
	buf.WriteString("\r\n")
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	if _, err := conn.Write(body); err != nil {
		return nil, err
	}
	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("waas read status: %w", err)
	}
	statusLine = strings.TrimSpace(statusLine)
	parts := strings.SplitN(statusLine, " ", 3)
	statusCode := 0
	if len(parts) >= 2 {
		statusCode, _ = strconv.Atoi(parts[1])
	}
	if statusCode == 0 {
		statusCode = http.StatusBadGateway
	}
	header := make(http.Header)
	var contentLength int64 = -1
	hasChunked := false
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("waas read headers: %w", err)
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		key := http.CanonicalHeaderKey(strings.TrimSpace(line[:i]))
		value := strings.TrimSpace(line[i+1:])
		if key == "Transfer-Encoding" {
			hasChunked = strings.EqualFold(value, "chunked") || strings.Contains(strings.ToLower(value), "chunked")
			header.Set(key, "chunked")
			continue
		}
		if key == "Content-Length" {
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				contentLength = n
			}
		}
		header.Add(key, value)
	}
	log.Printf("9PSB WaaS response: status=%d contentLength=%d hasChunked=%v", statusCode, contentLength, hasChunked)
	var respBody []byte
	if contentLength >= 0 {
		respBody = make([]byte, contentLength)
		if _, err := io.ReadFull(br, respBody); err != nil {
			log.Printf("9PSB WaaS read body error: %v", err)
			return nil, fmt.Errorf("waas read body: %w", err)
		}
	} else if hasChunked {
		respBody, err = readChunkedOrRaw(br)
		if err != nil {
			rest, _ := io.ReadAll(br)
			if len(rest) > 0 {
				log.Printf("9PSB WaaS chunked read failed (%v), raw body (%d bytes): %s", err, len(rest), truncate(rest, 500))
			} else {
				log.Printf("9PSB WaaS chunked read failed: %v (no remaining bytes)", err)
			}
			return nil, fmt.Errorf("waas read chunked body: %w", err)
		}
	} else {
		respBody, _ = io.ReadAll(br)
	}
	if len(respBody) > 0 {
		log.Printf("9PSB WaaS body (%d bytes): %s", len(respBody), truncate(respBody, 300))
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:    statusLine,
		Header:    header,
		Body:      io.NopCloser(bytes.NewReader(respBody)),
		Request:   req,
	}, nil
}

func truncate(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// readChunkedOrRaw reads the body. If headers said chunked but the body is actually raw JSON (9PSB quirk),
// we treat first line as start of body and read until EOF. Otherwise parse chunked encoding.
func readChunkedOrRaw(r *bufio.Reader) ([]byte, error) {
	peek, err := r.Peek(1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	// If first byte looks like JSON, server sent raw body (no chunk framing)
	if len(peek) > 0 && (peek[0] == '{' || peek[0] == '[') {
		all, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		return all, nil
	}
	line, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return append([]byte(line), readRemaining(r)...), nil
		}
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	trimmed := strings.TrimSpace(line)
	// If line looks like JSON start, treat whole response as raw body (this line + rest)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		rest := readRemaining(r)
		return append([]byte(line), rest...), nil
	}
	var chunkSize int64
	if _, err := fmt.Sscanf(trimmed, "%x", &chunkSize); err != nil {
		// Not a chunk size line; treat as raw body
		rest := readRemaining(r)
		return append([]byte(line), rest...), nil
	}
	// Proper chunked: read chunk payloads only (no chunk size lines in output)
	var out []byte
	for chunkSize > 0 {
		if int64(len(out))+chunkSize > 10*1024*1024 {
			return nil, fmt.Errorf("chunked body too large")
		}
		chunk := make([]byte, chunkSize)
		n, err := io.ReadFull(r, chunk)
		if err != nil {
			if err == io.EOF && n > 0 {
				out = append(out, chunk[:n]...)
				return out, nil
			}
			return nil, err
		}
		out = append(out, chunk...)
		if _, err := r.ReadSlice('\n'); err != nil && err != bufio.ErrBufferFull {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		line, err = r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed = strings.TrimSpace(line)
		if i := strings.Index(trimmed, ";"); i >= 0 {
			trimmed = trimmed[:i]
		}
		chunkSize = 0
		if _, err := fmt.Sscanf(trimmed, "%x", &chunkSize); err != nil {
			return nil, fmt.Errorf("invalid chunk size %q: %w", line, err)
		}
	}
	return out, nil
}

func readRemaining(r *bufio.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

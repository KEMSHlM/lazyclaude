package server

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 message types.

// Request represents a JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// IsNotification returns true if the request has no ID (notification).
func (r *Request) IsNotification() bool {
	return r.ID == nil || string(r.ID) == "null"
}

// NewResponse creates a success response.
func NewResponse(id json.RawMessage, result any) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates an error response.
func NewErrorResponse(id json.RawMessage, code int, message string) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
}

// ParseRequest parses a JSON-RPC request from raw bytes.
// Returns an error if the jsonrpc version is not "2.0".
func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	if req.JSONRPC != "2.0" {
		return nil, fmt.Errorf("unsupported jsonrpc version: %q", req.JSONRPC)
	}
	return &req, nil
}

// MarshalResponse serializes a JSON-RPC response to bytes.
func MarshalResponse(resp Response) ([]byte, error) {
	return json.Marshal(resp)
}
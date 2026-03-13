package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseErrorResult(t *testing.T, result *mcp.CallToolResult) map[string]interface{} {
	t.Helper()
	textContent := result.Content[0].(mcp.TextContent)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(textContent.Text), &resp); err != nil {
		t.Fatalf("failed to parse error JSON %q: %v", textContent.Text, err)
	}
	return resp
}

// TestHandleError_WrappedConnectError verifies that a connect.Error wrapped
// by fmt.Errorf preserves the original connect error code rather than
// falling through to UNKNOWN.
func TestHandleError_WrappedConnectError(t *testing.T) {
	inner := connect.NewError(connect.CodeNotFound, errors.New("resource missing"))
	wrapped := fmt.Errorf("service layer: %w", inner)

	result, handleErr := HandleError(wrapped)
	if handleErr != nil {
		t.Fatalf("HandleError returned unexpected error: %v", handleErr)
	}
	if result == nil {
		t.Fatal("HandleError returned nil result")
	}

	resp := parseErrorResult(t, result)

	if resp["code"] != "NOT_FOUND" {
		t.Errorf("wrapped connect.Error code: got %q, want %q", resp["code"], "NOT_FOUND")
	}
	// After fix: wrapper context is preserved in the message
	msg := resp["message"].(string)
	if !strings.Contains(msg, "service layer") || !strings.Contains(msg, "resource missing") {
		t.Errorf("wrapped connect.Error message should contain wrapper context: got %q", msg)
	}
}

// TestHandleError_DoubleWrappedConnectError tests a connect.Error buried
// under two layers of wrapping.
func TestHandleError_DoubleWrappedConnectError(t *testing.T) {
	inner := connect.NewError(connect.CodePermissionDenied, errors.New("forbidden"))
	wrapped := fmt.Errorf("middleware: %w", fmt.Errorf("handler: %w", inner))

	result, handleErr := HandleError(wrapped)
	if handleErr != nil {
		t.Fatalf("HandleError returned unexpected error: %v", handleErr)
	}
	if result == nil {
		t.Fatal("HandleError returned nil result")
	}

	resp := parseErrorResult(t, result)

	if resp["code"] != "PERMISSION_DENIED" {
		t.Errorf("double-wrapped connect.Error code: got %q, want %q", resp["code"], "PERMISSION_DENIED")
	}
	// After fix: wrapper context is preserved in the message
	msg := resp["message"].(string)
	if !strings.Contains(msg, "middleware") || !strings.Contains(msg, "forbidden") {
		t.Errorf("double-wrapped connect.Error message should contain wrapper context: got %q", msg)
	}
}

// TestHandleError_WrappedGRPCStatus tests that wrapping a gRPC status error
// preserves the code.
func TestHandleError_WrappedGRPCStatus(t *testing.T) {
	grpcErr := status.Error(codes.NotFound, "entity not found")
	wrapped := fmt.Errorf("service layer: %w", grpcErr)

	result, handleErr := HandleError(wrapped)
	if handleErr != nil {
		t.Fatalf("HandleError returned unexpected error: %v", handleErr)
	}
	if result == nil {
		t.Fatal("HandleError returned nil result")
	}

	resp := parseErrorResult(t, result)

	if resp["code"] != "NOT_FOUND" {
		t.Errorf("wrapped gRPC error code: got %q, want %q", resp["code"], "NOT_FOUND")
	}
}

// TestHandleError_JoinedConnectAndPlainError tests errors.Join with a
// connect.Error and a plain error. errors.As should still find the
// connect.Error inside a joined error.
func TestHandleError_JoinedConnectAndPlainError(t *testing.T) {
	connectErr := connect.NewError(connect.CodeUnavailable, errors.New("service down"))
	plainErr := errors.New("additional context")
	joined := errors.Join(connectErr, plainErr)

	result, handleErr := HandleError(joined)
	if handleErr != nil {
		t.Fatalf("HandleError returned unexpected error: %v", handleErr)
	}
	if result == nil {
		t.Fatal("HandleError returned nil result")
	}

	resp := parseErrorResult(t, result)

	if resp["code"] != "UNAVAILABLE" {
		t.Errorf("joined connect.Error code: got %q, want %q", resp["code"], "UNAVAILABLE")
	}
}

// TestHandleError_WrappedConnectErrorMessagePreservation checks whether the
// wrapper context ("service layer: ...") is preserved or lost. When
// errors.As unwraps to the inner connect.Error, the wrapper message is
// discarded. This may or may not be desired, but it's the current behavior
// we want to document.
func TestHandleError_WrappedConnectErrorMessagePreservation(t *testing.T) {
	inner := connect.NewError(connect.CodeInternal, errors.New("db timeout"))
	wrapped := fmt.Errorf("important context about the failure: %w", inner)

	result, _ := HandleError(wrapped)
	resp := parseErrorResult(t, result)

	// The wrapper message "important context about the failure" is lost
	// because HandleError extracts the inner connect.Error via errors.As
	// and calls connectErr.Message() which only returns "db timeout".
	//
	// This is arguably a bug: the additional context from the wrapping
	// is silently discarded. The full err.Error() would be:
	//   "important context about the failure: internal: db timeout"
	// but resp["message"] will just be "db timeout".
	fullMessage := wrapped.Error()
	if resp["message"] == fullMessage {
		// If this passes, the wrapper context IS preserved (unexpected)
		t.Log("wrapper context preserved (good)")
	} else if resp["message"] == "db timeout" {
		// Wrapper context is lost - this is the likely behavior and a real
		// information loss bug
		t.Errorf("wrapper context lost: HandleError discards wrapping message.\n"+
			"  got message:  %q\n"+
			"  full error:   %q\n"+
			"  This means callers who add context via fmt.Errorf will have "+
			"that context silently dropped.", resp["message"], fullMessage)
	} else {
		t.Errorf("unexpected message: got %q", resp["message"])
	}
}

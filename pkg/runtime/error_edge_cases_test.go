package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandleError_AllGRPCCodes(t *testing.T) {
	codesToTest := []struct {
		code     codes.Code
		expected string
	}{
		{codes.Canceled, "CANCELLED"},
		{codes.Unknown, "UNKNOWN"},
		{codes.InvalidArgument, "INVALID_ARGUMENT"},
		{codes.DeadlineExceeded, "DEADLINE_EXCEEDED"},
		{codes.NotFound, "NOT_FOUND"},
		{codes.AlreadyExists, "ALREADY_EXISTS"},
		{codes.PermissionDenied, "PERMISSION_DENIED"},
		{codes.ResourceExhausted, "RESOURCE_EXHAUSTED"},
		{codes.FailedPrecondition, "FAILED_PRECONDITION"},
		{codes.Aborted, "ABORTED"},
		{codes.OutOfRange, "OUT_OF_RANGE"},
		{codes.Unimplemented, "UNIMPLEMENTED"},
		{codes.Internal, "INTERNAL"},
		{codes.Unavailable, "UNAVAILABLE"},
		{codes.DataLoss, "DATA_LOSS"},
		{codes.Unauthenticated, "UNAUTHENTICATED"},
	}

	for _, tt := range codesToTest {
		t.Run(tt.expected, func(t *testing.T) {
			g := NewWithT(t)

			st := status.New(tt.code, "test error for "+tt.expected)
			result, err := HandleError(st.Err())
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).ToNot(BeNil())
			g.Expect(result.IsError).To(BeTrue())

			textContent := result.Content[0].(mcp.TextContent)
			var errorResp map[string]any
			g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
			g.Expect(errorResp["code"]).To(Equal(tt.expected))
			g.Expect(errorResp["message"]).To(Equal("test error for " + tt.expected))
		})
	}
}

func TestHandleError_ConnectErrorCodes(t *testing.T) {
	connectCodes := []struct {
		code     connect.Code
		expected string
	}{
		{connect.CodeCanceled, "CANCELLED"},
		{connect.CodeNotFound, "NOT_FOUND"},
		{connect.CodeAlreadyExists, "ALREADY_EXISTS"},
		{connect.CodePermissionDenied, "PERMISSION_DENIED"},
		{connect.CodeUnauthenticated, "UNAUTHENTICATED"},
		{connect.CodeUnimplemented, "UNIMPLEMENTED"},
		{connect.CodeInternal, "INTERNAL"},
		{connect.CodeUnavailable, "UNAVAILABLE"},
	}

	for _, tt := range connectCodes {
		t.Run(tt.expected, func(t *testing.T) {
			g := NewWithT(t)

			connectErr := connect.NewError(tt.code, errors.New("connect: "+tt.expected))
			result, err := HandleError(connectErr)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).ToNot(BeNil())
			g.Expect(result.IsError).To(BeTrue())

			textContent := result.Content[0].(mcp.TextContent)
			var errorResp map[string]any
			g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
			g.Expect(errorResp["code"]).To(Equal(tt.expected))
		})
	}
}

func TestHandleError_WrappedError(t *testing.T) {
	g := NewWithT(t)

	// A plain Go error wrapped with fmt.Errorf
	inner := errors.New("root cause")
	wrapped := fmt.Errorf("context: %w", inner)

	result, err := HandleError(wrapped)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
	g.Expect(errorResp["code"]).To(Equal("UNKNOWN"))
	g.Expect(errorResp["message"]).To(Equal("context: root cause"))
}

func TestHandleError_GRPCWithMultipleDetails(t *testing.T) {
	g := NewWithT(t)

	st := status.New(codes.InvalidArgument, "multiple issues")

	badReq := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{Field: "name", Description: "name is required"},
		},
	}
	precondition := &errdetails.PreconditionFailure{
		Violations: []*errdetails.PreconditionFailure_Violation{
			{Type: "TOS", Subject: "user", Description: "Terms not accepted"},
		},
	}

	st, err := st.WithDetails(badReq, precondition)
	g.Expect(err).ToNot(HaveOccurred())

	result, handleErr := HandleError(st.Err())
	g.Expect(handleErr).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())

	details, ok := errorResp["details"].([]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(details).To(HaveLen(2))

	// First detail should be BadRequest
	d0 := details[0].(map[string]any)
	g.Expect(d0["@type"]).To(ContainSubstring("BadRequest"))

	// Second detail should be PreconditionFailure
	d1 := details[1].(map[string]any)
	g.Expect(d1["@type"]).To(ContainSubstring("PreconditionFailure"))
}

func TestHandleError_GRPCWithResourceInfo(t *testing.T) {
	g := NewWithT(t)

	st := status.New(codes.NotFound, "resource not found")
	resourceInfo := &errdetails.ResourceInfo{
		ResourceType: "item",
		ResourceName: "item-123",
		Owner:        "user-456",
		Description:  "The requested item does not exist",
	}

	st, err := st.WithDetails(resourceInfo)
	g.Expect(err).ToNot(HaveOccurred())

	result, handleErr := HandleError(st.Err())
	g.Expect(handleErr).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())

	details := errorResp["details"].([]any)
	g.Expect(details).To(HaveLen(1))

	detail := details[0].(map[string]any)
	g.Expect(detail["@type"]).To(ContainSubstring("ResourceInfo"))
}

func TestHandleError_GRPCWithErrorInfo(t *testing.T) {
	g := NewWithT(t)

	st := status.New(codes.FailedPrecondition, "quota exceeded")
	errorInfo := &errdetails.ErrorInfo{
		Reason: "QUOTA_EXCEEDED",
		Domain: "myservice.example.com",
		Metadata: map[string]string{
			"limit":    "100",
			"consumed": "150",
		},
	}

	st, err := st.WithDetails(errorInfo)
	g.Expect(err).ToNot(HaveOccurred())

	result, handleErr := HandleError(st.Err())
	g.Expect(handleErr).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())

	details := errorResp["details"].([]any)
	g.Expect(details).To(HaveLen(1))

	detail := details[0].(map[string]any)
	g.Expect(detail["@type"]).To(ContainSubstring("ErrorInfo"))
}

func TestHandleError_EmptyMessage(t *testing.T) {
	g := NewWithT(t)

	st := status.New(codes.Internal, "")
	result, err := HandleError(st.Err())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
	g.Expect(errorResp["code"]).To(Equal("INTERNAL"))
	// Message might be empty string or absent depending on StatusToNice behavior
	if msg, ok := errorResp["message"]; ok {
		g.Expect(msg).To(BeEmpty())
	}
}

func TestHandleError_LongErrorMessage(t *testing.T) {
	g := NewWithT(t)

	longMsg := strings.Repeat("x", 10000)
	st := status.New(codes.Internal, longMsg)
	result, err := HandleError(st.Err())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
	g.Expect(errorResp["message"]).To(Equal(longMsg))
}

func TestHandleError_SpecialCharsInMessage(t *testing.T) {
	g := NewWithT(t)

	specialMsg := `Error with "quotes" and \backslashes\ and <html> and 日本語`
	st := status.New(codes.Internal, specialMsg)
	result, err := HandleError(st.Err())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
	g.Expect(errorResp["message"]).To(Equal(specialMsg))
}

func TestHandleError_GRPCNoDetails(t *testing.T) {
	g := NewWithT(t)

	// gRPC error without any details attached
	st := status.New(codes.NotFound, "not found")
	result, err := HandleError(st.Err())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())

	textContent := result.Content[0].(mcp.TextContent)
	var errorResp map[string]any
	g.Expect(json.Unmarshal([]byte(textContent.Text), &errorResp)).To(Succeed())
	g.Expect(errorResp["code"]).To(Equal("NOT_FOUND"))
	// No details field or empty details
}

func TestHandleError_PlainErrorFormatsAsJSON(t *testing.T) {
	g := NewWithT(t)

	// Even a plain Go error should be formatted as structured JSON
	result, err := HandleError(errors.New("something broke"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())
	g.Expect(result.IsError).To(BeTrue())

	// Should be valid JSON
	textContent := result.Content[0].(mcp.TextContent)
	g.Expect(json.Valid([]byte(textContent.Text))).To(BeTrue())
}

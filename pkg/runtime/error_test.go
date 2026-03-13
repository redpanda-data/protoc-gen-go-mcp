package runtime

import (
	"encoding/json"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestHandleError_SimpleError(t *testing.T) {
	err := errors.New("simple error")
	result, handleErr := HandleError(err)

	if handleErr != nil {
		t.Fatalf("HandleError should not return an error, got: %v", handleErr)
	}

	if result == nil {
		t.Fatal("HandleError should return a result")
	}

	// Parse the JSON error response
	var errorResp map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result.Text), &errorResp); jsonErr != nil {
		t.Fatalf("Failed to parse error JSON: %v", jsonErr)
	}

	if errorResp["code"] != "UNKNOWN" {
		t.Errorf("Expected code 'UNKNOWN', got: %s", errorResp["code"])
	}

	if errorResp["message"] != "simple error" {
		t.Errorf("Expected message 'simple error', got: %s", errorResp["message"])
	}
}

func TestHandleError_GRPCStatus(t *testing.T) {
	// Create a gRPC status error with details
	st := status.New(codes.InvalidArgument, "invalid request")

	// Add a detail message
	stringValue := wrapperspb.String("additional info")

	st, err := st.WithDetails(stringValue)
	if err != nil {
		t.Fatalf("Failed to add details: %v", err)
	}

	result, handleErr := HandleError(st.Err())

	if handleErr != nil {
		t.Fatalf("HandleError should not return an error, got: %v", handleErr)
	}

	if result == nil {
		t.Fatal("HandleError should return a result")
	}

	// Parse the JSON error response
	var errorResp map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result.Text), &errorResp); jsonErr != nil {
		t.Fatalf("Failed to parse error JSON: %v", jsonErr)
	}

	if errorResp["code"] != "INVALID_ARGUMENT" {
		t.Errorf("Expected code 'INVALID_ARGUMENT', got: %s", errorResp["code"])
	}

	if errorResp["message"] != "invalid request" {
		t.Errorf("Expected message 'invalid request', got: %s", errorResp["message"])
	}

	details, hasDetails := errorResp["details"].([]interface{})
	if !hasDetails || len(details) == 0 {
		t.Error("Expected error details, got none")
	}

	// Verify the actual content of error details
	if len(details) > 0 {
		detail := details[0].(map[string]interface{})

		// Check @type field
		if typeField, ok := detail["@type"]; ok {
			if typeStr, ok := typeField.(string); ok {
				if typeStr != "type.googleapis.com/google.protobuf.StringValue" {
					t.Errorf("Expected @type 'type.googleapis.com/google.protobuf.StringValue', got: %s", typeStr)
				}
			}
		} else {
			t.Error("Expected @type field in error details")
		}

		// Check value field
		if valueField, ok := detail["value"]; ok {
			if valueStr, ok := valueField.(string); ok {
				if valueStr != "additional info" {
					t.Errorf("Expected value 'additional info', got: %s", valueStr)
				}
			}
		} else {
			t.Error("Expected value field in error details")
		}
	}
}

func TestHandleError_ConnectError(t *testing.T) {
	// Create a Connect error
	connectErr := connect.NewError(connect.CodeInvalidArgument, errors.New("connect error"))

	result, handleErr := HandleError(connectErr)

	if handleErr != nil {
		t.Fatalf("HandleError should not return an error, got: %v", handleErr)
	}

	if result == nil {
		t.Fatal("HandleError should return a result")
	}

	// Parse the JSON error response
	var errorResp map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result.Text), &errorResp); jsonErr != nil {
		t.Fatalf("Failed to parse error JSON: %v", jsonErr)
	}

	if errorResp["code"] != "INVALID_ARGUMENT" {
		t.Errorf("Expected code 'INVALID_ARGUMENT', got: %s", errorResp["code"])
	}

	if errorResp["message"] != "connect error" {
		t.Errorf("Expected message 'connect error', got: %s", errorResp["message"])
	}
}

func TestHandleError_GRPCStatusWithBadRequest(t *testing.T) {
	// Create a gRPC status error with BadRequest details
	st := status.New(codes.InvalidArgument, "validation failed")

	// Add BadRequest details with field violations
	badRequest := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{
				Field:       "email",
				Description: "email is required",
			},
			{
				Field:       "password",
				Description: "password must be at least 8 characters",
			},
		},
	}

	st, err := st.WithDetails(badRequest)
	if err != nil {
		t.Fatalf("Failed to add details: %v", err)
	}

	result, handleErr := HandleError(st.Err())

	if handleErr != nil {
		t.Fatalf("HandleError should not return an error, got: %v", handleErr)
	}

	if result == nil {
		t.Fatal("HandleError should return a result")
	}

	// Parse the JSON error response
	var errorResp map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(result.Text), &errorResp); jsonErr != nil {
		t.Fatalf("Failed to parse error JSON: %v", jsonErr)
	}

	if errorResp["code"] != "INVALID_ARGUMENT" {
		t.Errorf("Expected code 'INVALID_ARGUMENT', got: %s", errorResp["code"])
	}

	if errorResp["message"] != "validation failed" {
		t.Errorf("Expected message 'validation failed', got: %s", errorResp["message"])
	}

	details, hasDetails := errorResp["details"].([]interface{})
	if !hasDetails || len(details) == 0 {
		t.Error("Expected error details, got none")
	}

	// Verify the BadRequest error details content
	if len(details) > 0 {
		detail := details[0].(map[string]interface{})

		// Check @type field
		if typeField, ok := detail["@type"]; ok {
			if typeStr, ok := typeField.(string); ok {
				if typeStr != "type.googleapis.com/google.rpc.BadRequest" {
					t.Errorf("Expected @type 'type.googleapis.com/google.rpc.BadRequest', got: %s", typeStr)
				}
			}
		} else {
			t.Error("Expected @type field in error details")
		}

		// Check fieldViolations field (protojson uses camelCase by default)
		if fieldViolationsField, ok := detail["fieldViolations"]; ok {
			if violations, ok := fieldViolationsField.([]interface{}); ok {
				if len(violations) != 2 {
					t.Errorf("Expected 2 field violations, got: %d", len(violations))
				}

				// Check first violation
				if len(violations) > 0 {
					violation := violations[0].(map[string]interface{})
					if field, ok := violation["field"].(string); ok {
						if field != "email" {
							t.Errorf("Expected field 'email', got: %s", field)
						}
					}
					if desc, ok := violation["description"].(string); ok {
						if desc != "email is required" {
							t.Errorf("Expected description 'email is required', got: %s", desc)
						}
					}
				}

				// Check second violation
				if len(violations) > 1 {
					violation := violations[1].(map[string]interface{})
					if field, ok := violation["field"].(string); ok {
						if field != "password" {
							t.Errorf("Expected field 'password', got: %s", field)
						}
					}
					if desc, ok := violation["description"].(string); ok {
						if desc != "password must be at least 8 characters" {
							t.Errorf("Expected description 'password must be at least 8 characters', got: %s", desc)
						}
					}
				}
			}
		} else {
			t.Error("Expected fieldViolations field in error details")
		}
	}
}

func TestHandleError_NilError(t *testing.T) {
	result, handleErr := HandleError(nil)

	if handleErr != nil {
		t.Fatalf("HandleError should not return an error for nil input, got: %v", handleErr)
	}

	if result != nil {
		t.Fatal("HandleError should return nil result for nil error")
	}
}

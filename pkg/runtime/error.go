// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/mark3labs/mcp-go/mcp"
	apierrors "github.com/redpanda-data/common-go/api/errors"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// HandleError converts a gRPC/Connect error into a structured MCP tool result
// It extracts error codes, messages, and detailed error information using common-go utilities
func HandleError(err error) (*mcp.CallToolResult, error) {
	if err == nil {
		return nil, nil
	}

	// Convert to google.rpc.Status regardless of source
	var statusProto *spb.Status

	// Try to extract gRPC status first
	if st, ok := status.FromError(err); ok {
		statusProto = st.Proto()
	} else if connectErr := new(connect.Error); errors.As(err, &connectErr) {
		// Handle Connect RPC errors using ConnectErrorToGoogleStatus
		statusProto = apierrors.ConnectErrorToGoogleStatus(connectErr)
		// Preserve wrapper context: if the error was wrapped (e.g., fmt.Errorf),
		// the outer message is lost by ConnectErrorToGoogleStatus which only sees
		// the inner connect.Error. Use the full error chain message instead.
		if fullMsg := err.Error(); fullMsg != connectErr.Error() {
			statusProto.Message = fullMsg
		}
	} else {
		// Create a basic status for generic errors
		statusProto = &spb.Status{
			Code:    int32(codes.Unknown),
			Message: err.Error(),
			Details: nil,
		}
	}

	// Use StatusToNice to convert to the common-go ErrorStatus format
	niceStatus := apierrors.StatusToNice(statusProto)

	// Marshal the niceStatus directly to JSON using protojson
	finalJSON, marshalErr := protojson.Marshal(niceStatus)
	if marshalErr != nil {
		// Fallback to simple error message if JSON marshaling fails
		return mcp.NewToolResultError("Error: " + err.Error()), nil
	}

	return mcp.NewToolResultError(string(finalJSON)), nil
}

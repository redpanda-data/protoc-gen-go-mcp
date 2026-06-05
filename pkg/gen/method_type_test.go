package gen

import (
	"encoding/json"
	"strings"
	"testing"

	enumv1 "go.lixinio.com/apis/lixin/common/enum/v1"
	mcpv1 "go.lixinio.com/apis/lixin/common/mcp/v1"
	validatorv1 "go.lixinio.com/apis/lixin/common/validator/v1"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestToolForMethod_STDListAddsDescriptions(t *testing.T) {
	method := testMethodDescriptor(t, false, true)
	tool := ToolForMethod(method, "List things")

	inputSchema := decodeSchema(t, tool.RawInputSchema)
	outputSchema := decodeSchema(t, tool.RawOutputSchema)

	assertPropertyHasDescription(t, inputSchema, "parent", "资源类型")
	assertPropertyHasDescription(t, inputSchema, "parent", "groups/{group_id}/stores/{store_id}")
	assertPropertyHasDescription(t, inputSchema, "filter", "flow_type")
	assertPropertyHasDescription(t, inputSchema, "filter", "=(int64/int64s)")
	assertPropertyHasDescription(t, inputSchema, "filter", "【枚举 EnergyFlowType】")
	assertPropertyHasDescription(t, inputSchema, "filter", "1001 ENERGY_FLOW_TYPE_AUCTION_DEAL")
	assertPropertyHasDescription(t, inputSchema, "filter", "拍卖成交奖励")
	assertPropertyHasDescription(t, inputSchema, "filter", "【示例】\n- flow_type = 1001\n- flow_type = (1001, 1002)")
	assertPropertyNotContains(t, inputSchema, "filter", " eq ")
	assertPropertyNotContains(t, inputSchema, "filter", "ENERGY_FLOW_TYPE_UNSPECIFIED")
	assertPropertyHasDescription(t, inputSchema, "order_by", "create_time")
	assertPropertyHasDescription(t, inputSchema, "page_token", "next_page_token")
	assertPropertyHasDescription(t, outputSchema, "next_page_token", "page_token")
	assertPropertyHasDescription(t, outputSchema, "total_size", "总记录数")
}

func TestToolForMethod_FieldDescPriorityOverMethodType(t *testing.T) {
	method := testMethodDescriptor(t, true, true)
	tool := ToolForMethod(method, "List things")
	inputSchema := decodeSchema(t, tool.RawInputSchema)
	got := propertyDescription(t, inputSchema, "filter")
	want := "自定义 filter 描述"
	if got != want {
		t.Fatalf("filter description mismatch, got=%q want=%q", got, want)
	}
}

func TestBuildFilterExampleSliceSyntax(t *testing.T) {
	desc := &validatorv1.FilterDescriptor{
		Rules: []*validatorv1.FieldRules{
			{
				Field: "status",
				Ops: []*validatorv1.FieldOpRule{
					{Op: "eq", Types: []string{"int64", "int64s"}},
				},
			},
			{
				Field: "name",
				Ops: []*validatorv1.FieldOpRule{
					{Op: "eq", Types: []string{"strs"}},
				},
			},
		},
	}
	got := buildFilterExampleLines(desc, nil)
	want := []string{
		"status = 1",
		"status = (1, 2)",
		"name = (demo1, demo2)",
		"status = 1 AND name = (demo1, demo2)",
	}
	if len(got) != len(want) {
		t.Fatalf("buildFilterExampleLines() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildFilterExampleLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilterOpToSyntax(t *testing.T) {
	tests := []struct {
		ruleOp string
		want   string
	}{
		{"eq", "="},
		{"ne", "!="},
		{"gt", ">"},
		{"gte", ">="},
		{"lt", "<"},
		{"lte", "<="},
		{"like", "<>"},
		{"llike", "<:"},
		{"rlike", ":>"},
		{"has", ":"},
		{"unknown", "unknown"},
		{"  eq  ", "="},
	}
	for _, tc := range tests {
		t.Run(tc.ruleOp, func(t *testing.T) {
			if got := filterOpToSyntax(tc.ruleOp); got != tc.want {
				t.Fatalf("filterOpToSyntax(%q) = %q, want %q", tc.ruleOp, got, tc.want)
			}
		})
	}
}

func TestToolForMethod_NoMethodTypeNoEnhancement(t *testing.T) {
	method := testMethodDescriptor(t, false, false)
	tool := ToolForMethod(method, "List things")
	inputSchema := decodeSchema(t, tool.RawInputSchema)
	got := propertyDescription(t, inputSchema, "filter")
	if got != "" {
		t.Fatalf("expected empty filter description without method_type, got=%q", got)
	}
}

func testMethodDescriptor(t *testing.T, withFieldDesc bool, withMethodType bool) protoreflect.MethodDescriptor {
	t.Helper()

	filterOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(filterOpts, validatorv1.E_Filter, &validatorv1.FilterDescriptor{
		Object: "test.method_type.EnergyFlow",
		Rules: []*validatorv1.FieldRules{
			{
				Field: "flow_type",
				Ops: []*validatorv1.FieldOpRule{
					{Op: "eq", Types: []string{"int64", "int64s"}},
				},
			},
		},
	})
	if withFieldDesc {
		proto.SetExtension(filterOpts, mcpv1.E_FieldDesc, "自定义 filter 描述")
	}

	orderOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(orderOpts, validatorv1.E_Order, &validatorv1.OrderDescriptor{
		Object: "test.method_type.EnergyFlow",
		Rules: []*validatorv1.OrderRule{
			{Field: "create_time", Direction: "desc"},
		},
	})

	parentOpts := &descriptorpb.FieldOptions{}
	proto.SetExtension(parentOpts, annotations.E_ResourceReference, &annotations.ResourceReference{
		Type: "v1.store.dos.lixinio.com/Store",
	})

	methodOpts := &descriptorpb.MethodOptions{}
	if withMethodType {
		proto.SetExtension(methodOpts, mcpv1.E_MethodType, mcpv1.MethodType_METHOD_TYPE_STD_LIST)
	}

	resourceOpts := &descriptorpb.MessageOptions{}
	proto.SetExtension(resourceOpts, annotations.E_Resource, &annotations.ResourceDescriptor{
		Type:    "v1.store.dos.lixinio.com/Store",
		Pattern: []string{"groups/{group_id}/stores/{store_id}"},
	})

	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test_method_type.proto"),
		Package: proto.String("test.method_type"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Req"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("parent"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Options: parentOpts},
					{Name: proto.String("page_size"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
					{Name: proto.String("page_token"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: proto.String("filter"), Number: proto.Int32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Options: filterOpts},
					{Name: proto.String("order_by"), Number: proto.Int32(5), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Options: orderOpts},
				},
			},
			{
				Name: proto.String("Resp"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("next_page_token"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: proto.String("total_size"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
				},
			},
			{
				Name:    proto.String("Store"),
				Options: resourceOpts,
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("name"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
				},
			},
			{
				Name: proto.String("EnergyFlow"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("flow_type"),
						Number: proto.Int32(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".test.method_type.EnergyFlowType"),
					},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("EnergyFlowType"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{
						Name:    proto.String("ENERGY_FLOW_TYPE_UNSPECIFIED"),
						Number:  proto.Int32(0),
					},
					{
						Name:    proto.String("ENERGY_FLOW_TYPE_AUCTION_DEAL"),
						Number:  proto.Int32(1001),
						Options: enumValueOptions(t, "拍卖成交奖励"),
					},
					{
						Name:    proto.String("ENERGY_FLOW_TYPE_STORE_UNSOLD_MATCH_DEAL"),
						Number:  proto.Int32(1002),
						Options: enumValueOptions(t, "流拍撮合成交奖励"),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("DemoService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("ListThings"),
						InputType:  proto.String(".test.method_type.Req"),
						OutputType: proto.String(".test.method_type.Resp"),
						Options:    methodOpts,
					},
				},
			},
		},
	}

	fd, err := protodesc.NewFile(fdProto, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile failed: %v", err)
	}
	return fd.Services().Get(0).Methods().Get(0)
}

func decodeSchema(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json unmarshal schema failed: %v", err)
	}
	return out
}

func enumValueOptions(t *testing.T, name string) *descriptorpb.EnumValueOptions {
	t.Helper()
	opts := &descriptorpb.EnumValueOptions{}
	proto.SetExtension(opts, enumv1.E_Enum, &enumv1.EnumDescriptor{Name: name})
	return opts
}

func assertPropertyNotContains(t *testing.T, schema map[string]any, fieldName, substr string) {
	t.Helper()
	desc := propertyDescription(t, schema, fieldName)
	if strings.Contains(desc, substr) {
		t.Fatalf("field %s description=%q should not contain %q", fieldName, desc, substr)
	}
}

func assertPropertyHasDescription(t *testing.T, schema map[string]any, fieldName, contains string) {
	t.Helper()
	desc := propertyDescription(t, schema, fieldName)
	if !strings.Contains(desc, contains) {
		t.Fatalf("field %s description=%q does not contain %q", fieldName, desc, contains)
	}
}

func propertyDescription(t *testing.T, schema map[string]any, fieldName string) string {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %#v", schema)
	}
	field, ok := properties[fieldName].(map[string]any)
	if !ok {
		t.Fatalf("schema has no field %s", fieldName)
	}
	desc, _ := field["description"].(string)
	return desc
}

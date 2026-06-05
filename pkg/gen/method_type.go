package gen

import (
	"fmt"
	"sort"
	"strings"

	enumv1 "go.lixinio.com/apis/lixin/common/enum/v1"
	mcpv1 "go.lixinio.com/apis/lixin/common/mcp/v1"
	validatorv1 "go.lixinio.com/apis/lixin/common/validator/v1"
	"go.lixinio.com/fexp"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// validatorRuleOpToFexpSyntax maps validator FieldOpRule.op semantic codes to fexp filter syntax.
var validatorRuleOpToFexpSyntax = map[string]string{
	"eq":    string(fexp.OpEq),
	"ne":    string(fexp.OpNe),
	"gt":    string(fexp.OpGt),
	"gte":   string(fexp.OpGte),
	"lt":    string(fexp.OpLt),
	"lte":   string(fexp.OpLte),
	"like":  string(fexp.OpLike),
	"llike": string(fexp.OpLlike),
	"rlike": string(fexp.OpRlike),
	"has":   string(fexp.OpHas),
}

func filterOpToSyntax(op string) string {
	if syntax, ok := validatorRuleOpToFexpSyntax[strings.TrimSpace(op)]; ok {
		return syntax
	}
	return op
}

// validatorSliceValueTypes are ValueType codes (const.proto) for fexp set/slice values.
var validatorSliceValueTypes = map[string]bool{
	"strs":     true,
	"float32s": true,
	"float64s": true,
	"bools":    true,
	"int8s":    true,
	"uint8s":   true,
	"int16s":   true,
	"uint16s":  true,
	"int32s":   true,
	"uint32s":  true,
	"int64s":   true,
	"uint64s":  true,
}

func isValidatorSliceValueType(valueType string) bool {
	return validatorSliceValueTypes[strings.TrimSpace(valueType)]
}

func firstValidatorScalarValueType(types []string) string {
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" || isValidatorSliceValueType(t) {
			continue
		}
		return t
	}
	return ""
}

func firstValidatorSliceValueType(types []string) string {
	for _, t := range types {
		t = strings.TrimSpace(t)
		if isValidatorSliceValueType(t) {
			return t
		}
	}
	return ""
}

func methodType(method protoreflect.MethodDescriptor) mcpv1.MethodType {
	opts := method.Options()
	if opts == nil || !proto.HasExtension(opts, mcpv1.E_MethodType) {
		return mcpv1.MethodType_METHOD_TYPE_UNSPECIFIED
	}
	t, ok := proto.GetExtension(opts, mcpv1.E_MethodType).(mcpv1.MethodType)
	if !ok {
		return mcpv1.MethodType_METHOD_TYPE_UNSPECIFIED
	}
	return t
}

func applyMethodType(method protoreflect.MethodDescriptor, inputSchema, outputSchema map[string]any) {
	switch methodType(method) {
	case mcpv1.MethodType_METHOD_TYPE_STD_LIST:
		applyStdList(method, inputSchema, outputSchema)
	}
}

func applyStdList(method protoreflect.MethodDescriptor, inputSchema, outputSchema map[string]any) {
	reqFields := method.Input().Fields()

	setPropertyDescriptionIfEmpty(inputSchema, "parent", describeParent(reqFields.ByName("parent"), method))
	setPropertyDescriptionIfEmpty(inputSchema, "page_size", "【用途】控制本次返回条数。\n【规则】遵循 AIP-158 分页约定；建议使用合理值避免单次返回过大。\n【示例】\"page_size\": 10")
	setPropertyDescriptionIfEmpty(inputSchema, "page_token", "【用途】请求下一页数据。\n【规则】首页可留空；翻页时传入上一次响应的 next_page_token（AIP-158）。\n【示例】\"page_token\": \"CAEQASIIZGEzOWEzZWU\"")
	setPropertyDescriptionIfEmpty(inputSchema, "filter", describeFilter(reqFields.ByName("filter"), method))
	setPropertyDescriptionIfEmpty(inputSchema, "order_by", describeOrderBy(reqFields.ByName("order_by")))

	setPropertyDescriptionIfEmpty(outputSchema, "next_page_token", "【用途】下一页游标。\n【规则】非空表示还有下一页，请将其作为下一次请求的 page_token；为空表示已到末页。")
	setPropertyDescriptionIfEmpty(outputSchema, "total_size", "【用途】当前过滤条件下的总记录数。\n【规则】该值可能大于当前页实际返回条数。")
}

func setPropertyDescriptionIfEmpty(schema map[string]any, fieldName, desc string) {
	if desc == "" || schema == nil {
		return
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	propRaw, ok := properties[fieldName]
	if !ok {
		return
	}
	prop, ok := propRaw.(map[string]any)
	if !ok {
		return
	}
	if existing, ok := prop["description"].(string); ok && strings.TrimSpace(existing) != "" {
		return
	}
	prop["description"] = CleanComment(desc)
}

func describeParent(fd protoreflect.FieldDescriptor, method protoreflect.MethodDescriptor) string {
	if fd == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("【用途】指定列表查询所属的父级资源。\n")
	b.WriteString("【规则】请按资源名（AIP-122）填写完整 parent。")

	opts := fd.Options()
	if opts != nil && proto.HasExtension(opts, annotations.E_ResourceReference) {
		ref, ok := proto.GetExtension(opts, annotations.E_ResourceReference).(*annotations.ResourceReference)
		if ok && ref != nil && strings.TrimSpace(ref.GetType()) != "" {
			resourceType := ref.GetType()
			b.WriteString("\n【资源类型】")
			b.WriteString(resourceType)
			patterns := lookupResourcePatterns(method.ParentFile(), resourceType)
			if len(patterns) > 0 {
				b.WriteString("\n【可用 pattern】")
				b.WriteString(strings.Join(patterns, "；"))
				b.WriteString("\n【示例】按上述 pattern 填写完整 parent 路径。")
			}
		}
	}
	return b.String()
}

func describeFilter(fd protoreflect.FieldDescriptor, method protoreflect.MethodDescriptor) string {
	if fd == nil {
		return ""
	}
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, validatorv1.E_Filter) {
		return "【用途】按条件筛选列表结果。\n【规则】遵循 AIP-160/fexp 语法，使用字段 + 操作符表达过滤条件。"
	}
	filterDesc, ok := proto.GetExtension(opts, validatorv1.E_Filter).(*validatorv1.FilterDescriptor)
	if !ok || filterDesc == nil {
		return ""
	}
	objectMD := resolveMessageByFullName(methodDescriptorRoots(method), filterDesc.GetObject())
	lines := []string{
		"【用途】按条件筛选列表结果。",
		"【规则】遵循 AIP-160/fexp 语法；仅可使用下述字段和操作符。",
	}
	for _, rule := range filterDesc.GetRules() {
		fieldName := rule.GetField()
		if strings.TrimSpace(fieldName) == "" {
			continue
		}
		ops := make([]string, 0, len(rule.GetOps()))
		for _, op := range rule.GetOps() {
			syntaxOp := filterOpToSyntax(op.GetOp())
			types := strings.Join(op.GetTypes(), "/")
			if types == "" {
				ops = append(ops, syntaxOp)
				continue
			}
			ops = append(ops, fmt.Sprintf("%s(%s)", syntaxOp, types))
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", fieldName, strings.Join(ops, ", ")))
		if enumDesc := describeEnumForFilterRule(objectMD, fieldName); enumDesc != "" {
			lines = append(lines, enumDesc)
		}
	}

	if examples := buildFilterExampleLines(filterDesc, objectMD); len(examples) > 0 {
		lines = append(lines, "【示例】")
		for _, example := range examples {
			lines = append(lines, "- "+example)
		}
	}
	return strings.Join(lines, "\n")
}

func describeEnumForFilterRule(objectMD protoreflect.MessageDescriptor, fieldPath string) string {
	fd := resolveFieldByPath(objectMD, fieldPath)
	if fd == nil || fd.Kind() != protoreflect.EnumKind {
		return ""
	}
	return describeEnumForFilter(fd.Enum())
}

func describeEnumForFilter(ed protoreflect.EnumDescriptor) string {
	if ed == nil {
		return ""
	}
	var values []string
	for i := 0; i < ed.Values().Len(); i++ {
		ev := ed.Values().Get(i)
		if shouldSkipEnumValue(ev) {
			continue
		}
		label := enumValueLabel(ev)
		values = append(values, fmt.Sprintf("  - %d %s：%s", ev.Number(), ev.Name(), label))
	}
	if len(values) == 0 {
		return ""
	}
	return fmt.Sprintf("  【枚举 %s】filter 请使用数值：\n%s", ed.Name(), strings.Join(values, "\n"))
}

func shouldSkipEnumValue(ev protoreflect.EnumValueDescriptor) bool {
	if ev.Number() == 0 {
		return true
	}
	return strings.HasSuffix(string(ev.Name()), "_UNSPECIFIED")
}

func enumValueLabel(ev protoreflect.EnumValueDescriptor) string {
	opts := ev.Options()
	if opts != nil && proto.HasExtension(opts, enumv1.E_Enum) {
		desc, ok := proto.GetExtension(opts, enumv1.E_Enum).(*enumv1.EnumDescriptor)
		if ok && desc != nil && strings.TrimSpace(desc.GetName()) != "" {
			return desc.GetName()
		}
	}
	return string(ev.Name())
}

func filterEnumNumbers(objectMD protoreflect.MessageDescriptor, fieldPath string, limit int) []int32 {
	fd := resolveFieldByPath(objectMD, fieldPath)
	if fd == nil || fd.Kind() != protoreflect.EnumKind || limit <= 0 {
		return nil
	}
	numbers := make([]int32, 0, limit)
	for i := 0; i < fd.Enum().Values().Len(); i++ {
		ev := fd.Enum().Values().Get(i)
		if shouldSkipEnumValue(ev) {
			continue
		}
		numbers = append(numbers, int32(ev.Number()))
		if len(numbers) >= limit {
			break
		}
	}
	return numbers
}

func formatFilterScalarExampleValue(objectMD protoreflect.MessageDescriptor, fieldPath, valueType string) string {
	if enumNums := filterEnumNumbers(objectMD, fieldPath, 1); len(enumNums) > 0 {
		return fmt.Sprintf("%d", enumNums[0])
	}
	switch strings.TrimSpace(valueType) {
	case "int64", "int32", "uint64", "uint32", "int16", "uint16", "int8", "uint8":
		return "1"
	case "float32", "float64":
		return "1.0"
	case "bool":
		return "true"
	case "time":
		return `"2006-01-02 15:04:05"`
	case "date":
		return `"2006-01-02"`
	default:
		return "demo"
	}
}

func formatFilterSliceExampleValue(objectMD protoreflect.MessageDescriptor, fieldPath, valueType string) string {
	if enumNums := filterEnumNumbers(objectMD, fieldPath, 2); len(enumNums) > 0 {
		items := make([]string, 0, len(enumNums))
		for _, n := range enumNums {
			items = append(items, fmt.Sprintf("%d", n))
		}
		if len(items) == 1 {
			items = append(items, items[0])
		}
		return formatFexpSetValue(items)
	}
	switch strings.TrimSpace(valueType) {
	case "int64s", "int32s", "uint64s", "uint32s", "int16s", "uint16s", "int8s", "uint8s":
		return formatFexpSetValue([]string{"1", "2"})
	case "float32s", "float64s":
		return formatFexpSetValue([]string{"1.0", "2.0"})
	case "bools":
		return formatFexpSetValue([]string{"true", "false"})
	case "strs":
		return formatFexpSetValue([]string{"demo1", "demo2"})
	default:
		return formatFexpSetValue([]string{"1", "2"})
	}
}

// formatFexpSetValue renders fexp set syntax: (v1, v2). See go.lixinio.com/fexp parse_test.go.
func formatFexpSetValue(items []string) string {
	if len(items) == 0 {
		return "()"
	}
	return "(" + strings.Join(items, ", ") + ")"
}

func formatFilterExampleClause(fieldName, op, value string) string {
	if op == string(fexp.OpHas) {
		return fieldName + op + value
	}
	return fmt.Sprintf("%s %s %s", fieldName, op, value)
}

func buildFilterExampleClausesForRule(
	rule *validatorv1.FieldRules,
	objectMD protoreflect.MessageDescriptor,
) []string {
	fieldName := strings.TrimSpace(rule.GetField())
	if fieldName == "" || len(rule.GetOps()) == 0 {
		return nil
	}
	opRule := rule.GetOps()[0]
	op := string(fexp.OpEq)
	if opVal := strings.TrimSpace(opRule.GetOp()); opVal != "" {
		op = filterOpToSyntax(opVal)
	}

	var clauses []string
	if scalarType := firstValidatorScalarValueType(opRule.GetTypes()); scalarType != "" {
		value := formatFilterScalarExampleValue(objectMD, fieldName, scalarType)
		clauses = append(clauses, formatFilterExampleClause(fieldName, op, value))
	}
	if sliceType := firstValidatorSliceValueType(opRule.GetTypes()); sliceType != "" {
		value := formatFilterSliceExampleValue(objectMD, fieldName, sliceType)
		clauses = append(clauses, formatFilterExampleClause(fieldName, op, value))
	}
	if len(clauses) == 0 {
		clauses = append(clauses, formatFilterExampleClause(fieldName, op, "demo"))
	}
	return clauses
}

func primaryFilterExampleClauseForRule(
	rule *validatorv1.FieldRules,
	objectMD protoreflect.MessageDescriptor,
) string {
	clauses := buildFilterExampleClausesForRule(rule, objectMD)
	if len(clauses) == 0 {
		return ""
	}
	return clauses[0]
}

func buildFilterExampleLines(desc *validatorv1.FilterDescriptor, objectMD protoreflect.MessageDescriptor) []string {
	if desc == nil || len(desc.GetRules()) == 0 {
		return nil
	}
	var (
		lines        []string
		combineParts []string
	)
	for _, rule := range desc.GetRules() {
		clauses := buildFilterExampleClausesForRule(rule, objectMD)
		lines = append(lines, clauses...)
		if primary := primaryFilterExampleClauseForRule(rule, objectMD); primary != "" {
			combineParts = append(combineParts, primary)
		}
	}
	if len(combineParts) >= 2 {
		lines = append(lines, strings.Join(combineParts, " AND "))
	}
	return lines
}

func describeOrderBy(fd protoreflect.FieldDescriptor) string {
	if fd == nil {
		return ""
	}
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, validatorv1.E_Order) {
		return "【用途】控制返回顺序。\n【规则】遵循 AIP-132，格式为 \"field asc|desc\"，多个字段可用逗号分隔。"
	}
	orderDesc, ok := proto.GetExtension(opts, validatorv1.E_Order).(*validatorv1.OrderDescriptor)
	if !ok || orderDesc == nil {
		return ""
	}
	lines := []string{
		"【用途】控制返回顺序。",
		"【规则】遵循 AIP-132，格式为 \"field asc|desc\"，多个字段用逗号分隔。",
	}
	example := ""
	for _, rule := range orderDesc.GetRules() {
		fieldName := strings.TrimSpace(rule.GetField())
		if fieldName == "" {
			continue
		}
		direction := strings.TrimSpace(rule.GetDirection())
		if direction == "" {
			direction = "asc/desc"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", fieldName, direction))
		if example == "" {
			if direction == "asc/desc" {
				example = fieldName + " desc"
			} else {
				example = fieldName + " " + direction
			}
		}
	}
	if example != "" {
		lines = append(lines, "【示例】"+example)
	}
	return strings.Join(lines, "\n")
}

func collectDescriptorFiles(root protoreflect.FileDescriptor) []protoreflect.FileDescriptor {
	if root == nil {
		return nil
	}
	visited := map[string]bool{}
	var files []protoreflect.FileDescriptor
	var collect func(fd protoreflect.FileDescriptor)
	collect = func(fd protoreflect.FileDescriptor) {
		if fd == nil {
			return
		}
		p := fd.Path()
		if visited[p] {
			return
		}
		visited[p] = true
		files = append(files, fd)
		for i := 0; i < fd.Imports().Len(); i++ {
			collect(fd.Imports().Get(i).FileDescriptor)
		}
	}
	collect(root)
	return files
}

func methodDescriptorRoots(method protoreflect.MethodDescriptor) []protoreflect.FileDescriptor {
	if method == nil {
		return nil
	}
	roots := []protoreflect.FileDescriptor{method.ParentFile(), method.Input().ParentFile()}
	seen := map[string]bool{}
	out := make([]protoreflect.FileDescriptor, 0, len(roots))
	for _, root := range roots {
		if root == nil {
			continue
		}
		p := root.Path()
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, root)
	}
	return out
}

func filterObjectNameCandidates(fullName string) []string {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil
	}
	candidates := []string{fullName}
	normalized := strings.ReplaceAll(fullName, "-", "_")
	if normalized != fullName {
		candidates = append(candidates, normalized)
	}
	return candidates
}

func resolveMessageByFullName(roots []protoreflect.FileDescriptor, fullName string) protoreflect.MessageDescriptor {
	files := collectDescriptorFilesFromRoots(roots)
	for _, candidate := range filterObjectNameCandidates(fullName) {
		target := protoreflect.FullName(candidate)
		for _, fd := range files {
			if md := findMessageByFullName(fd.Messages(), target); md != nil {
				return md
			}
		}
	}
	return nil
}

func collectDescriptorFilesFromRoots(roots []protoreflect.FileDescriptor) []protoreflect.FileDescriptor {
	visited := map[string]bool{}
	var files []protoreflect.FileDescriptor
	for _, root := range roots {
		for _, fd := range collectDescriptorFiles(root) {
			p := fd.Path()
			if visited[p] {
				continue
			}
			visited[p] = true
			files = append(files, fd)
		}
	}
	return files
}

func findMessageByFullName(messages protoreflect.MessageDescriptors, target protoreflect.FullName) protoreflect.MessageDescriptor {
	for i := 0; i < messages.Len(); i++ {
		md := messages.Get(i)
		if md.FullName() == target {
			return md
		}
		if nested := findMessageByFullName(md.Messages(), target); nested != nil {
			return nested
		}
	}
	return nil
}

func resolveFieldByPath(md protoreflect.MessageDescriptor, path string) protoreflect.FieldDescriptor {
	path = strings.TrimSpace(path)
	if md == nil || path == "" {
		return nil
	}
	segments := strings.Split(path, ".")
	current := md
	for i, segment := range segments {
		if current == nil {
			return nil
		}
		fd := current.Fields().ByName(protoreflect.Name(segment))
		if fd == nil {
			return nil
		}
		if i == len(segments)-1 {
			return fd
		}
		if fd.Kind() != protoreflect.MessageKind {
			return nil
		}
		current = fd.Message()
	}
	return nil
}

func lookupResourcePatterns(root protoreflect.FileDescriptor, resourceType string) []string {
	if root == nil || strings.TrimSpace(resourceType) == "" {
		return nil
	}
	patternSet := map[string]struct{}{}
	for _, fd := range collectDescriptorFiles(root) {
		for i := 0; i < fd.Messages().Len(); i++ {
			collectMessagePatterns(fd.Messages().Get(i), resourceType, patternSet)
		}
	}
	if len(patternSet) == 0 {
		return nil
	}
	out := make([]string, 0, len(patternSet))
	for pattern := range patternSet {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func collectMessagePatterns(md protoreflect.MessageDescriptor, resourceType string, out map[string]struct{}) {
	if md == nil {
		return
	}
	opts := md.Options()
	if opts != nil && proto.HasExtension(opts, annotations.E_Resource) {
		resource, ok := proto.GetExtension(opts, annotations.E_Resource).(*annotations.ResourceDescriptor)
		if ok && resource != nil && resource.GetType() == resourceType {
			for _, pattern := range resource.GetPattern() {
				if strings.TrimSpace(pattern) != "" {
					out[pattern] = struct{}{}
				}
			}
		}
	}
	for i := 0; i < md.Messages().Len(); i++ {
		collectMessagePatterns(md.Messages().Get(i), resourceType, out)
	}
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

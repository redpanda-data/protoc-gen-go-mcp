package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	mcpv1 "go.lixinio.com/apis/lixin/common/mcp/v1"
	v1 "go.lixinio.com/apis/lixin/dos/usedcar-bff/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	svc := v1.File_lixin_dos_usedcar_bff_v1_svc_proto.Services().ByName("UsedCarBffService")
	if svc == nil {
		fmt.Fprintln(os.Stderr, "UsedCarBffService not found")
		os.Exit(1)
	}
	meth := svc.Methods().ByName("ListEnergyFlows")
	if meth == nil {
		fmt.Fprintln(os.Stderr, "ListEnergyFlows not found")
		os.Exit(1)
	}

	comment := ""
	if opts := meth.Options(); opts != nil && proto.HasExtension(opts, mcpv1.E_MethodDesc) {
		if desc, ok := proto.GetExtension(opts, mcpv1.E_MethodDesc).(string); ok {
			comment = desc
		}
	}
	if strings.TrimSpace(comment) == "" {
		comment = "查看账户能量流水"
	}

	tool := gen.ToolForMethod(meth, comment)

	fmt.Println("=== Tool ===")
	fmt.Printf("Name: %s\n", tool.Name)
	fmt.Printf("Description:\n%s\n\n", tool.Description)

	fmt.Println("=== Input Schema ===")
	printSchema(tool.RawInputSchema)
	fmt.Println()

	fmt.Println("=== Output Schema ===")
	printSchema(tool.RawOutputSchema)
}

func printSchema(raw json.RawMessage) {
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal schema: %v\n", err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

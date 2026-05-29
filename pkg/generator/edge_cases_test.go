package generator

import (
	"bytes"
	"encoding/json"

	. "github.com/onsi/gomega"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// validateAgainstSchema compiles the schema and validates the JSON data against it.
func validateAgainstSchema(g *WithT, schemaJSON, dataJSON []byte) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertContent = true
	err := compiler.AddResource("schema.json", bytes.NewReader(schemaJSON))
	g.Expect(err).ToNot(HaveOccurred())
	schema, err := compiler.Compile("schema.json")
	g.Expect(err).ToNot(HaveOccurred())
	var data any
	err = json.Unmarshal(dataJSON, &data)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(schema.Validate(data)).To(Succeed())
}

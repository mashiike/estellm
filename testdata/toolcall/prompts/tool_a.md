{{ define "config" }}
{
    type: "test_agent", 
    description: "tool_a description",
    payload_schema: {
        type: "object",
        properties: {
            name: {
                type: "string"
            }
        },
        required: ["name"]
    }
}
{{ end }}

this is tool_a node.

{{ define "config" }}
{
    type: "test_agent", 
    tools: ["tool_a", "tool_b"],
    depends_on: ["start"]
}
{{ end }}

this is main node.

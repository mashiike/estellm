{{ define "config" }}
{
    type: "test_agent",  
}
{{ end }}

this is start node.
tool_a: {{ (ref `tool_a`).config.description }}
tool_b: {{ (ref `tool_b`).config.description }}

{{ define "config" }}
{
    type: "test_agent",
    tools:["external_tool"], 
}
{{ end }}

this is task_b node. 
<context>
{{ ref `start` }}
</context>

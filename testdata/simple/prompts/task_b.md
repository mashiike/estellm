{{ define "config" }}
{
    type: "test_agent", 
}
{{ end }}

this is task_b node. 
<context>
{{ ref `start` }}
</context>

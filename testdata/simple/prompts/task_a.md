{{ define "config" }}
{
    type: "test_agent", 
}
{{ end }}

this is task_a node. 
<context>
{{ ref `start` }}
</context>

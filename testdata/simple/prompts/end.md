{{ define "config" }}
{
    type: "test_agent", 
}
{{ end }}

this is end node. 
<context>
{{ ref `task_a` }}
{{ ref `task_b` }}
</context>

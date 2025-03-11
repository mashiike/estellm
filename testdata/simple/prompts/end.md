{{ define "config" }}
{
    type: "test_agent", 
    response_metadata: {
        key1: "value1",
        key2: "value2",
    },
}
{{ end }}

this is end node. 
<context>
{{ ref `task_a` }}
{{ ref `task_b` }}
</context>

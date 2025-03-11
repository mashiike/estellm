{{ define "config" }}
{
    type: "test_agent", 
    depends_on: ["task_a1"],
}
{{ end }}

this is task_a2 node.

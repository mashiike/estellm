{{ define "config" }}
{
    type: "test_agent", 
    depends_on: ["task_a2", "task_b2"],
}
{{ end }}

this is summary node.

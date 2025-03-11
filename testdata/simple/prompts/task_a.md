{{ define "config" }}
local mustEnv = std.native("mustEnv");
{
    type: "test_agent", 
    tools: [
        mustEnv("REMOTE_TOOL_ENDPOINT"),
    ],
}
{{ end }}

this is task_a node. 
<context>
{{ ref `start` }}
</context>

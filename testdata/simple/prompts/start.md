{{ define "config" }}
local mustEnv = std.native("mustEnv");
{

    type: "test_agent",
    as_reasoning: (mustEnv("AS_REASONING") == "true"),
}
{{ end }}

this is start node.

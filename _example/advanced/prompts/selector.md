{{ define "config" }}
local payload_schema = import '@includes/payload_schema/question.libsonnet';
{
    type: "decision",
    default: true, 
    model_provider: "bedrock",
    model_id: "anthropic.claude-3-haiku-20240307-v1:0",
    payload_schema: payload_schema, 
    fallback_agent: "standard",
    description: "Analyze the user's question and select the appropriate agent."
}
{{ end }}

Your task is to analyze the user's question and select the appropriate agent.

The answer should be in JSON format according to the schema below.
<output_schema>
{{ decisionSchema (dependentNames) | toJson }}
</output_schema>
Please write the reasoning in Japanese.

The available agents are as follows.
<agents>
{{- range $i, $v :=  dependentNames}}
{{- $conf := (get (dependents) $v).config }}
    <agent name="{{ $v }}"> {{ $conf.description }} </agent>
{{- end }}
</agents>

Start the answer immediately without any preamble and output only the correct JSON.
<role:user/> {{ .payload | toJson }}

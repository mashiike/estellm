{{ define "config" }}
{
    type: "constant", 
    description: "Fetches weather information.",
    publish: true,
    payload_schema: {
        type: "object",
        properties: {
            location: { 
                type: "string",
                example: "Tokyo",
                description: "The name of the location to fetch the weather for",
            },
        },
        required: ["location"],
    },  
    publish_types: ["tool"], 
}
{{ end }}

{{- if eq .payload.location "Tokyo" -}}
Cloudy
{{- else -}}
Sunny
{{- end -}}

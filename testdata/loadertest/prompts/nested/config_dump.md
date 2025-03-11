{{ define "config" }}
{
    name: "hoge",
    type: "test_agent", 
    key1: "value1",
    key2: "value2",  
    payload_schema: {
        type: "object"
    },
}
{{ end }}

{{  get (self.config) `key1` }}
{{  get (self.config) `key2` }}

{{ define "dummy_block" }}
{{ end }}

{{ define "config" }}
{
    type: "test_agent", 
}
{{ end }}

this is before1 node.
## dependents 
following nodes depend on this node:
{{ range $i, $v := dependentNames }}
{{ add $i 1 }}. name is {{$v}}, type is {{ (get (dependents) $v).config.type }}
{{- end }}

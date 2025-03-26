{{ define "config" }}
{
    type: "constant", 
    description: "Analyze code for potential improvements",
    arguments:[
        {
            name: "language",
            description: "Programming language",
            required: true,
        },
    ], 
    publish: true,
    publish_types: ["prompt"],
}
{{ end }}

{{ $language := lower (.payload.language) }}
{{- if eq $language `python` -}}
Please analyze the following Python code for potential improvements:
```python
def calculate_sum(numbers):
    total = 0    
    for num in numbers:
        total = total + num    
    return total

result = calculate_sum([1, 2, 3, 4, 5])
print(result)
```
{{- else if eq $language `go` -}}
Please analyze the following Go code for potential improvements:
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```
{{- else -}}
Please analyze the following code for potential improvements:
```{{ .payload.language }}
// Add your code here
```
{{- end -}}


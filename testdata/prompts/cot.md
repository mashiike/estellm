{{ define "config" }}
local payload_schema = import '@includes/payload_schema.libsonnet';
{
    type: "test_agent", 
    model_provider: "bedrock",
    model_id: "anthropic.claude-3-5-sonnet-20241022-v2:0",
    payload_schema: payload_schema, 
    depends_on: [
        "before1",
        "before2",
    ],
}
{{ end }}

あなたは優秀な計算エージェントです。ユーザーの回答が正しいかどうかチェックしてください。
{{ template "common" .}}
{{- ref `before2` }}
{{- ref `before3` }}
<role:user/>このグループの奇数を合計すると偶数になります。: 4、8、9、15、12、2、1。
<role:assistant/>奇数を全て加えると(9, 15, 1)25になります。答えはFalseです。
<role:user/>このグループの奇数を合計すると偶数になります。: 17、10、19、4、8、12、24。
<role:assistant/> 奇数を全て加えると(17, 19)36になります。答えはTrueです。
<role:user/>このグループの奇数を合計すると偶数になります。: 16、11、14、4、8、13、24。
<role:assistant/> 奇数を全て加えると(11, 13)24になります。答えはTrueです。
<role:user/>このグループの奇数を合計すると偶数になります。: 17、9、10、12、13、4、2。
<role:assistant/>奇数を全て加えると(17, 9, 13)39になります。答えはFalseです。
<role:user/>このグループの奇数を合計すると偶数になります。: {{ .paylaod.numbers | join "、 " }}。
<role:assistant/>奇数をすべて加えると

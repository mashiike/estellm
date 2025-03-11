package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/mashiike/estellm"
)

type getExchangeRateInput struct {
	BaseCurrency string `json:"base_currency" jsonschema:"title=base currency,description=The base currency,example=USD,default=USD"`
}

func main() {
	var port int
	flag.IntVar(&port, "port", 8088, "port number")
	flag.Parse()
	tool, err := estellm.NewTool(
		"get_exchange_rate",
		"Get current exchange rate",
		func(ctx context.Context, input getExchangeRateInput, w estellm.ResponseWriter) error {
			toolUseID, ok := estellm.ToolUseIDFromContext(ctx)
			if !ok {
				toolUseID = "<unknown>"
			}
			log.Printf("call get_exchange_rate tool: tool_use_id=%s, base_currency=%s", toolUseID, input.BaseCurrency)
			resp, err := http.Get("https://api.exchangerate-api.com/v4/latest/USD")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			bs, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			w.WritePart(estellm.TextPart(string(bs)))
			w.Finish(estellm.FinishReasonEndTurn, http.StatusText(resp.StatusCode))
			return nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	u, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		log.Fatal(err)
	}
	handler, err := estellm.NewRemoteToolHandler(estellm.RemoteToolHandlerConfig{
		Endpoint: u,
		Tool:     tool,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("start server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatal(err)
	}
}

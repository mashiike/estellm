package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mashiike/estellm"
	"github.com/mashiike/estellm/interanal/jsonutil"
	"github.com/mashiike/estellm/metadata"
	"github.com/sashabaranov/go-openai"
)

func init() {
	// Register the provider
	estellm.RegisterModelProvider("openai", &ModelProvider{})
}

type OpenAIClient interface {
	CreateImage(ctx context.Context, request openai.ImageRequest) (openai.ImageResponse, error)
	CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
}

type ModelProvider struct {
	init    sync.Once
	client  OpenAIClient
	initErr error
	apiKey  string
	baseURL string
}

func NewWithClient(client OpenAIClient) *ModelProvider {
	return &ModelProvider{client: client}
}

func (p *ModelProvider) SetClilent(client OpenAIClient) {
	p.client = client
}

func (p *ModelProvider) initClient() error {
	p.init.Do(func() {
		if p.client != nil {
			return
		}
		p.apiKey = os.Getenv("OPENAI_API_KEY")
		if p.apiKey == "" {
			p.initErr = errors.New("missing OPENAI_API_KEY")
			return
		}
		p.baseURL = os.Getenv("OPENAI_BASE_URL")
		config := openai.DefaultConfig(p.apiKey)
		if p.baseURL != "" {
			config.BaseURL = p.baseURL
		}
		p.client = openai.NewClientWithConfig(config)
	})
	return p.initErr
}

func (p *ModelProvider) newClient(modelParams map[string]any) (OpenAIClient, error) {
	var endpoint string
	if str, ok := modelParams["endpoint"].(string); ok {
		endpoint = str
	}
	if apiKey, ok := modelParams["api_key"].(string); ok && apiKey != "" {
		cfg := openai.DefaultConfig(apiKey)
		if endpoint != "" {
			cfg.BaseURL = endpoint
		}
		return openai.NewClientWithConfig(cfg), nil
	}
	if err := p.initClient(); err != nil {
		return nil, err
	}
	if endpoint != "" {
		config := openai.DefaultConfig(p.apiKey)
		config.BaseURL = endpoint
		return openai.NewClientWithConfig(config), nil
	}
	return p.client, nil
}

func (p *ModelProvider) GenerateText(ctx context.Context, req *estellm.GenerateTextRequest, w estellm.ResponseWriter) error {
	client, err := p.newClient(req.ModelParams)
	if err != nil {
		return fmt.Errorf("failed to create openai client: %w", err)
	}
	var input openai.ChatCompletionRequest
	if err := jsonutil.Remarshal(req.ModelParams, &input); err != nil {
		return fmt.Errorf("remarshal completion request: %w", err)
	}
	input.Model = req.ModelID
	input.Stream = true
	for key := range req.Metadata {
		if value := req.Metadata.GetString(key); value != "" {
			input.Metadata[key] = value
		}
	}
	input.Messages = make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	if req.System != "" {
		input.Messages = append(input.Messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleDeveloper,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: req.System,
				},
			},
		})
	}
	for _, msg := range req.Messages {
		var role string
		switch msg.Role {
		case estellm.RoleUser:
			role = openai.ChatMessageRoleUser
		case estellm.RoleAssistant:
			role = openai.ChatMessageRoleAssistant
		default:
			return fmt.Errorf("unsupported role: %s", msg.Role)
		}
		parts := make([]openai.ChatMessagePart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch part.Type {
			case estellm.PartTypeText:
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: part.Text,
				})
			case estellm.PartTypeBinary:
				mediaType, _, err := mime.ParseMediaType(part.MIMEType)
				if err != nil {
					return fmt.Errorf("parse media type: %w", err)
				}
				switch {
				case strings.HasPrefix(mediaType, "image/"):
					part := openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: fmt.Sprintf("data:%s;base64,%s", part.MIMEType, base64.StdEncoding.EncodeToString(part.Data)),
						},
					}
					parts = append(parts, part)
				case strings.HasPrefix(mediaType, "text/"):
					parts = append(parts, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: string(part.Data),
					})
				default:
					return fmt.Errorf("unsupported media type: %s", part.MIMEType)
				}
			}
		}
		input.Messages = append(input.Messages, openai.ChatCompletionMessage{
			Role:         role,
			MultiContent: parts,
		})
	}
	if len(req.Tools) > 0 {
		input.Tools = make([]openai.Tool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			slog.Debug("tool spec", "name", tool.Name(), "description", tool.Description(), "input_schema", tool.InputSchema())
			openaiTool := openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  tool.InputSchema(),
				},
			}
			input.Tools = append(input.Tools, openaiTool)
		}
	}
	input.Stream = true
	return p.generateTextMultiTrun(ctx, client, input, w, req.Tools)
}

func (p *ModelProvider) generateTextMultiTrun(ctx context.Context, client OpenAIClient, input openai.ChatCompletionRequest, w estellm.ResponseWriter, toolSet estellm.ToolSet) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		output, err := client.CreateChatCompletionStream(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to create completion: %w", err)
		}
		m := w.Metadata()
		setToMetadta(m, output.GetRateLimitHeaders())
		for k, v := range output.Header() {
			if strings.HasPrefix(k, "Openai-") {
				m.SetStrings(k, v)
			}
		}
		var textBuilder strings.Builder
		var role string
		toolUses := make([]openai.ToolCall, 0, len(toolSet))
		var currentToolCall openai.ToolCall
		streamReader := func(output *openai.ChatCompletionStream) (bool, error) {
			defer output.Close()
			for {
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				default:
				}
				response, err := output.Recv()
				if errors.Is(err, io.EOF) {
					slog.Info("stream closed", "response", response)
					return false, nil
				}
				if err != nil {
					return false, fmt.Errorf("failed to receive completion: %w", err)
				}
				for _, choice := range response.Choices {
					if choice.FinishReason != "" {
						if choice.FinishReason != openai.FinishReasonFunctionCall && choice.FinishReason != openai.FinishReasonToolCalls {
							switch choice.FinishReason {
							case openai.FinishReasonContentFilter:
								w.Finish(estellm.FinishReasonContentFiltered, "content filter")
							case openai.FinishReasonStop:
								w.Finish(estellm.FinishReasonEndTurn, "stop")
							case openai.FinishReasonLength:
								w.Finish(estellm.FinishReasonMaxTokens, "length")
							default:
								w.Finish(estellm.FinishReasonEndTurn, string(choice.FinishReason))
							}
							return false, nil
						}
						toolUses = append(toolUses, currentToolCall)
						return true, nil
					}
					if choice.Delta.Role != "" {
						role = choice.Delta.Role
					}
					if choice.Delta.Content != "" {
						w.WritePart(estellm.TextPart(choice.Delta.Content))
						textBuilder.WriteString(choice.Delta.Content)
						continue
					}
					if len(choice.Delta.ToolCalls) > 0 {
						for _, toolCall := range choice.Delta.ToolCalls {
							if toolCall.ID != "" {
								if currentToolCall.ID != "" {
									toolUses = append(toolUses, currentToolCall)
								}
								currentToolCall = toolCall
								continue
							}
							if len(toolCall.Function.Arguments) > 0 {
								currentToolCall.Function.Arguments += toolCall.Function.Arguments
								continue
							}
						}
						continue
					}
					slog.DebugContext(ctx, "untrap choice", "choice", choice)
				}
			}
		}
		isToolUse, err := streamReader(output)
		if err != nil {
			return fmt.Errorf("stream reader: %w", err)
		}
		if !isToolUse {
			return nil
		}
		slog.DebugContext(ctx, "tool uses", "tool_uses", toolUses, "role", role, "text", textBuilder.String())
		msg := openai.ChatCompletionMessage{
			Role: role,
		}
		if textBuilder.Len() > 0 {
			msg.Content = textBuilder.String()
		}
		msg.ToolCalls = append(msg.ToolCalls, toolUses...)
		input.Messages = append(input.Messages, msg)
		var wg sync.WaitGroup
		var mu sync.Mutex
		errs := make([]error, 0, len(toolUses))
		wg.Add(len(toolUses))
		for _, _toolUse := range toolUses {
			go func(toolUse openai.ToolCall) {
				defer wg.Done()
				result, err := toolCall(ctx, toolSet, toolUse.ID, toolUse.Function.Name, toolUse.Function.Arguments)
				if err != nil {
					mu.Lock()
					slog.WarnContext(ctx, "tool call error", "error", err)
					errs = append(errs, fmt.Errorf("tool call[%s]: %w", toolUse.ID, err))
					mu.Unlock()
					return
				}
				mu.Lock()
				input.Messages = append(input.Messages, result)
				mu.Unlock()
			}(_toolUse)
		}
		wg.Wait()
		if len(errs) > 0 {
			return fmt.Errorf("tool call errors: %v", errs)
		}
	}
}

func newToolResultWithError(toolUseID string, err error) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolUseID,
		Content:    fmt.Sprintf("failed to call tool[%s]: %s", toolUseID, err.Error()),
	}
}

func newToolResultWithResponse(toolUseID string, response *estellm.Response) openai.ChatCompletionMessage {
	parts := make([]openai.ChatMessagePart, 0, len(response.Message.Parts))
	for _, part := range response.Message.Parts {
		switch part.Type {
		case estellm.PartTypeText:
			parts = append(parts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.Text,
			})
		case estellm.PartTypeBinary:
			mediaType, _, err := mime.ParseMediaType(part.MIMEType)
			if err != nil {
				return newToolResultWithError(toolUseID, fmt.Errorf("parse media type: %w", err))
			}
			switch {
			case strings.HasPrefix(mediaType, "image/"):
				part := openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: fmt.Sprintf("data:%s;base64,%s", part.MIMEType, base64.StdEncoding.EncodeToString(part.Data)),
					},
				}
				parts = append(parts, part)
			case strings.HasPrefix(mediaType, "text/"):
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: string(part.Data),
				})
			default:
				return newToolResultWithError(toolUseID, fmt.Errorf("unsupported media type: %s", part.MIMEType))
			}
		}
	}
	return openai.ChatCompletionMessage{
		Role:         openai.ChatMessageRoleTool,
		ToolCallID:   toolUseID,
		MultiContent: parts,
	}
}

func toolCall(ctx context.Context, tools estellm.ToolSet, toolUseID string, toolName string, input string) (openai.ChatCompletionMessage, error) {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return newToolResultWithError(toolUseID, fmt.Errorf("unmarshal input: %w", err)), nil
	}
	for _, tool := range tools {
		if tool.Name() == toolName {
			w := estellm.NewBatchResponseWriter()
			if err := tool.Call(ctx, v, w); err != nil {
				return newToolResultWithError(toolUseID, err), nil
			}
			return newToolResultWithResponse(toolUseID, w.Response()), nil
		}
	}
	return openai.ChatCompletionMessage{}, fmt.Errorf("tool not found: %s", toolName)
}

func (p *ModelProvider) GenerateImage(ctx context.Context, req *estellm.GenerateImageRequest, w estellm.ResponseWriter) error {
	client, err := p.newClient(req.ModelParams)
	if err != nil {
		return fmt.Errorf("failed to create openai client: %w", err)
	}
	imageReq := openai.ImageRequest{
		Size:    "1024x1024",
		Quality: "standard",
		N:       1,
	}
	if err := jsonutil.Remarshal(req.ModelParams, &imageReq); err != nil {
		return fmt.Errorf("remarshal image request: %w", err)
	}
	imageReq.Model = req.ModelID
	var sb strings.Builder
	enc := estellm.NewMessageEncoder(&sb)
	enc.SkipReasoning()
	enc.TextOnly()
	enc.NoRole()
	if err := enc.Encode(req.System, req.Messages); err != nil {
		return fmt.Errorf("encode messages: %w", err)
	}
	imageReq.Prompt = sb.String()

	w.WritePart(estellm.TextPart(fmt.Sprintf("<prompt type=\"provided\">%s</prompt>", imageReq.Prompt)))
	output, err := client.CreateImage(ctx, imageReq)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}
	if len(output.Data) == 0 {
		w.Finish(estellm.FinishReasonEndTurn, "no data")
		return nil
	}
	m := w.Metadata()
	setToMetadta(m, output.GetRateLimitHeaders())
	for k, v := range output.Header() {
		if strings.HasPrefix(k, "Openai-") {
			m.SetStrings(k, v)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(len(output.Data))
	contentParts := make([][]estellm.ContentPart, len(output.Data))
	errs := make([]error, 0, len(output.Data))
	for _index, _data := range output.Data {
		go func(index int, data openai.ImageResponseDataInner) {
			defer wg.Done()
			parts := make([]estellm.ContentPart, 0, 2)
			if data.RevisedPrompt != "" {
				parts = append(parts, estellm.TextPart(fmt.Sprintf("<prompt type=\"revised\">%s</prompt>", data.RevisedPrompt)))
			}
			if data.B64JSON != "" {
				bs, err := base64.StdEncoding.DecodeString(data.B64JSON)
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("failed to decode b64json[%d]: %w", index, err))
					mu.Unlock()
					return
				}
				parts = append(parts, estellm.BinaryPart(http.DetectContentType(bs), bs))
				mu.Lock()
				contentParts[index] = parts
				mu.Unlock()
				return
			}
			resp, err := http.Get(data.URL)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to get image[%d]: %w", index, err))
				mu.Unlock()
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to get image[%d]: %s", index, resp.Status))
				mu.Unlock()
				return
			}
			bs, err := io.ReadAll(resp.Body)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to read body[%d]: %w", index, err))
				mu.Unlock()
				return
			}
			parts = append(parts, estellm.BinaryPart(resp.Header.Get("Content-Type"), bs))
			mu.Lock()
			contentParts[index] = parts
			mu.Unlock()
		}(_index, _data)
	}
	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("failed to get images: %v", errs)
	}
	w.WriteRole(estellm.RoleAssistant)
	for _, parts := range contentParts {
		if err := w.WritePart(parts...); err != nil {
			return fmt.Errorf("write part: %w", err)
		}
	}
	w.Finish(estellm.FinishReasonEndTurn, fmt.Sprintf("created %d images", len(output.Data)))
	return nil
}

func setToMetadta(m metadata.Metadata, h openai.RateLimitHeaders) {
	m.SetInt64("Openai-RateLimit-Remaining-Tokens", int64(h.RemainingTokens))
	m.SetInt64("Openai-RateLimit-Remaining-Requests", int64(h.RemainingRequests))
	m.SetInt64("Openai-RateLimit-Reset-Requests", h.ResetRequests.Time().Unix())
	m.SetInt64("Openai-RateLimit-Reset-Tokens", h.ResetTokens.Time().Unix())
	m.SetInt64("Openai-RateLimit-Limit-Tokens", int64(h.LimitTokens))
	m.SetInt64("Openai-RateLimit-Limit-Requests", int64(h.LimitRequests))
}

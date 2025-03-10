package bedrock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"mime"
	"regexp"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/mashiike/estellm"
	"github.com/mashiike/estellm/metadata"
)

func init() {
	// Register the provider
	estellm.RegisterModelProvider("bedrock", &ModelProvider{})
}

type BedrockAPIClient interface {
	ConverseStream(ctx context.Context, params *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
}

type ModelProvider struct {
	init    sync.Once
	awsCfg  *aws.Config
	client  BedrockAPIClient
	initErr error
}

func NewWithClient(client BedrockAPIClient) *ModelProvider {
	return &ModelProvider{client: client}
}

func (p *ModelProvider) SetClilent(client BedrockAPIClient) {
	p.client = client
}

func (p *ModelProvider) initClient() error {
	p.init.Do(func() {
		if p.client != nil {
			return
		}
		if p.awsCfg == nil {
			awsCfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				p.initErr = err
				return
			}
			p.awsCfg = &awsCfg
		}
		p.client = bedrockruntime.NewFromConfig(*p.awsCfg)
	})
	return p.initErr
}

func (p *ModelProvider) GenerateText(ctx context.Context, req *estellm.GenerateTextRequest, w estellm.ResponseWriter) error {
	if err := p.initClient(); err != nil {
		return err
	}
	input := &bedrockruntime.ConverseStreamInput{
		ModelId: aws.String(req.ModelID),
	}
	if req.System != "" {
		input.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{
				Value: req.System,
			},
		}
	}
	documentCount := 0
	for _, msg := range req.Messages {
		var tMsg types.Message
		switch msg.Role {
		case estellm.RoleUser:
			tMsg.Role = types.ConversationRoleUser
		case estellm.RoleAssistant:
			tMsg.Role = types.ConversationRoleAssistant
		default:
			return estellm.ErrInvalidMessageRole
		}
		for _, part := range msg.Parts {
			switch part.Type {
			case estellm.PartTypeText:
				tMsg.Content = append(tMsg.Content, &types.ContentBlockMemberText{
					Value: part.Text,
				})
			case estellm.PartTypeBinary:
				mediaType, _, err := mime.ParseMediaType(part.MIMEType)
				if err != nil {
					return fmt.Errorf("parse media type: %w", err)
				}
				switch {
				case strings.HasPrefix(mediaType, "image/"):
					var format types.ImageFormat
					switch mediaType {
					case "image/jpeg":
						format = types.ImageFormatJpeg
					case "image/png":
						format = types.ImageFormatPng
					case "image/gif":
						format = types.ImageFormatGif
					case "image/webp":
						format = types.ImageFormatWebp
					default:
						return fmt.Errorf("unsupported image format: %s", mediaType)
					}
					tMsg.Content = append(tMsg.Content, &types.ContentBlockMemberImage{
						Value: types.ImageBlock{
							Format: format,
							Source: &types.ImageSourceMemberBytes{
								Value: part.Data,
							},
						},
					})
				case strings.HasPrefix(mediaType, "text/"):
					tMsg.Content = append(tMsg.Content, &types.ContentBlockMemberText{
						Value: string(part.Data),
					})
				case mediaType == "application/pdf":
					tMsg.Content = append(tMsg.Content, &types.ContentBlockMemberDocument{
						Value: types.DocumentBlock{
							Format: types.DocumentFormatPdf,
							Name:   aws.String(fmt.Sprintf("document%d", documentCount)),
							Source: &types.DocumentSourceMemberBytes{
								Value: part.Data,
							},
						},
					})
					documentCount++
				default:
					return fmt.Errorf("unsupported binary content type: %s", mediaType)
				}
			default:
				return fmt.Errorf("unsupported content type: %s", part.Type)
			}
		}
		input.Messages = append(input.Messages, tMsg)
	}
	params := make(map[string]any, len(req.ModelParams))
	maps.Copy(params, req.ModelParams)
	if maxTokens, ok := req.ModelParams["max_tokens"]; ok {
		if input.InferenceConfig == nil {
			input.InferenceConfig = &types.InferenceConfiguration{}
		}
		input.InferenceConfig.MaxTokens = aws.Int32(toNumber[int32](maxTokens))
		delete(params, "max_tokens")
	}
	if temperature, ok := req.ModelParams["temperature"]; ok {
		if input.InferenceConfig == nil {
			input.InferenceConfig = &types.InferenceConfiguration{}
		}
		input.InferenceConfig.Temperature = aws.Float32(toNumber[float32](temperature))
		delete(params, "temperature")
	}
	if stopWords, ok := req.ModelParams["stop_words"].([]string); ok {
		if input.InferenceConfig == nil {
			input.InferenceConfig = &types.InferenceConfiguration{}
		}
		input.InferenceConfig.StopSequences = stopWords
		delete(params, "stop_words")
	}
	if len(params) > 0 {
		input.AdditionalModelRequestFields = document.NewLazyDocument(params)
	}
	if len(req.Metadata) > 0 {
		input.RequestMetadata = make(map[string]string)
		for _, k := range req.Metadata.Keys() {
			input.RequestMetadata[k] = req.Metadata.GetString(k)
		}
	}
	if len(req.Tools) > 0 {
		input.ToolConfig = &types.ToolConfiguration{
			Tools: make([]types.Tool, 0, len(req.Tools)),
		}
		for _, tool := range req.Tools {
			slog.Debug("tool spec", "name", tool.Name(), "description", tool.Description(), "input_schema", tool.InputSchema())
			input.ToolConfig.Tools = append(input.ToolConfig.Tools, &types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String(NormalizeToolName(tool.Name())),
					Description: aws.String(tool.Description()),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: document.NewLazyDocument(tool.InputSchema()),
					},
				},
			})
		}
	}
	return p.generateTextSingleTurn(ctx, input, w, req.Tools)
}

var (
	toolNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
)

func NormalizeToolName(input string) string {
	normalized := toolNameRe.ReplaceAllString(input, "_")
	normalized = strings.Trim(normalized, "_")
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	if normalized == "" {
		return "default_tool"
	}
	return normalized
}

func (p *ModelProvider) generateTextSingleTurn(ctx context.Context, input *bedrockruntime.ConverseStreamInput, w estellm.ResponseWriter, tools estellm.ToolSet) error {
	slog.DebugContext(ctx, "converse stream", "input", input)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		slog.DebugContext(ctx, "call converse stream")
		output, err := p.client.ConverseStream(ctx, input)
		if err != nil {
			return fmt.Errorf("converse stream: %w", err)
		}
		slog.DebugContext(ctx, "converse stream output tailing", "result_metadta", output.ResultMetadata)
		var msg types.Message
		var currentContent types.ContentBlock
		var toolInputBuilder bytes.Buffer
		for o := range output.GetStream().Events() {
			switch v := o.(type) {
			case *types.ConverseStreamOutputMemberContentBlockStart:
				cb, err := processContentBlockStart(ctx, v, w, &toolInputBuilder)
				if err != nil {
					return fmt.Errorf("process content block start: %w", err)
				}
				currentContent = cb
			case *types.ConverseStreamOutputMemberContentBlockDelta:
				cb, err := processContentBlockDelta(ctx, v, w, &toolInputBuilder)
				if err != nil {
					return fmt.Errorf("process content block delta: %w", err)
				}
				currentContent = mergeContentBlock(currentContent, cb)
			case *types.ConverseStreamOutputMemberContentBlockStop:
				if toolInputBuilder.Len() > 0 {
					var input any
					if err := json.Unmarshal(toolInputBuilder.Bytes(), &input); err != nil {
						return fmt.Errorf("unmarshal tool input: %w", err)
					}
					currentContent = mergeContentBlock(currentContent, &types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							Input: document.NewLazyDocument(input),
						},
					})
				}
				if currentContent != nil {
					msg.Content = append(msg.Content, currentContent)
					currentContent = nil
				}
			case *types.ConverseStreamOutputMemberMessageStart:
				if err := processMessageStart(ctx, v, w); err != nil {
					return fmt.Errorf("process message start: %w", err)
				}
				msg.Role = v.Value.Role
			case *types.ConverseStreamOutputMemberMetadata:
				setToMetadata(v.Value, w.Metadata())
				slog.DebugContext(ctx, "metadata updated", "value", v.Value)
			case *types.ConverseStreamOutputMemberMessageStop:
				slog.Debug("message complete", "message", msg)
				toolUse, err := processMessageStop(ctx, v, w)
				if err != nil {
					return fmt.Errorf("process message stop: %w", err)
				}
				if !toolUse {
					return nil
				}
			default:
				slog.DebugContext(ctx, "unknown event", "type", fmt.Sprintf("%T", o))
			}
		}
		input.Messages = append(input.Messages, msg)
		toolUses, err := extructToolUse(msg)
		if err != nil {
			return fmt.Errorf("extract tool use: %w", err)
		}
		slog.DebugContext(ctx, "tool use", "tool_uses", toolUses)
		toolResultMsg := types.Message{
			Role: types.ConversationRoleUser,
		}
		for _, toolUse := range toolUses {
			cb, err := toolCall(ctx, tools, toolUse.toolUseID, toolUse.toolName, toolUse.input)
			if err != nil {
				cb = newToolResultWithError(toolUse.toolUseID, err)
			}
			toolResultMsg.Content = append(toolResultMsg.Content, cb)
		}
		slog.DebugContext(ctx, "tool result", "message", toolResultMsg)
		input.Messages = append(input.Messages, toolResultMsg)
	}
}

type toolUse struct {
	toolUseID string
	toolName  string
	input     any
}

func extructToolUse(msg types.Message) ([]toolUse, error) {
	toolUses := make([]toolUse, 0, len(msg.Content))
	for _, cb := range msg.Content {
		if cb, ok := cb.(*types.ContentBlockMemberToolUse); ok {
			bs, err := cb.Value.Input.MarshalSmithyDocument()
			if err != nil {
				return nil, fmt.Errorf("marshal tool input: %w", err)
			}
			var input any
			if err := json.Unmarshal(bs, &input); err != nil {
				return nil, fmt.Errorf("unmarshal tool input: %w", err)
			}
			toolUses = append(toolUses, toolUse{
				toolUseID: *cb.Value.ToolUseId,
				toolName:  *cb.Value.Name,
				input:     input,
			})
		}
	}
	if len(toolUses) == 0 {
		return nil, errors.New("tool use not found")
	}
	return toolUses, nil
}

func newToolResultWithError(toolUseID string, err error) types.ContentBlock {
	return &types.ContentBlockMemberToolResult{
		Value: types.ToolResultBlock{
			ToolUseId: aws.String(toolUseID),
			Status:    types.ToolResultStatusError,
			Content: []types.ToolResultContentBlock{
				&types.ToolResultContentBlockMemberText{
					Value: fmt.Sprintf("error: %s", err),
				},
			},
		},
	}
}

func newToolResultWithResponse(toolUseID string, response *estellm.Response) types.ContentBlock {
	content := make([]types.ToolResultContentBlock, 0, len(response.Message.Parts))
	for _, part := range response.Message.Parts {
		switch part.Type {
		case estellm.PartTypeText:
			content = append(content, &types.ToolResultContentBlockMemberText{
				Value: part.Text,
			})
		case estellm.PartTypeBinary:
			mediaType, _, err := mime.ParseMediaType(part.MIMEType)
			if err != nil {
				return newToolResultWithError(toolUseID, fmt.Errorf("tool result parse media type: %w", err))
			}
			switch {
			case strings.HasPrefix(mediaType, "image/"):
				var format types.ImageFormat
				switch mediaType {
				case "image/jpeg":
					format = types.ImageFormatJpeg
				case "image/png":
					format = types.ImageFormatPng
				case "image/gif":
					format = types.ImageFormatGif
				case "image/webp":
					format = types.ImageFormatWebp
				default:
					return newToolResultWithError(toolUseID, fmt.Errorf("tool result unsupported image format: %s", mediaType))
				}
				content = append(content, &types.ToolResultContentBlockMemberImage{
					Value: types.ImageBlock{
						Format: format,
						Source: &types.ImageSourceMemberBytes{
							Value: part.Data,
						},
					},
				})
			case mediaType == "text/html":
				content = append(content, &types.ToolResultContentBlockMemberDocument{
					Value: types.DocumentBlock{
						Format: types.DocumentFormatHtml,
						Name:   aws.String("document"),
						Source: &types.DocumentSourceMemberBytes{
							Value: part.Data,
						},
					},
				})
			case strings.HasPrefix(mediaType, "text/"):
				content = append(content, &types.ToolResultContentBlockMemberText{
					Value: string(part.Data),
				})
			case mediaType == "application/pdf":
				content = append(content, &types.ToolResultContentBlockMemberDocument{
					Value: types.DocumentBlock{
						Format: types.DocumentFormatPdf,
						Name:   aws.String("document"),
						Source: &types.DocumentSourceMemberBytes{
							Value: part.Data,
						},
					},
				})
			default:
				return newToolResultWithError(toolUseID, fmt.Errorf("tool result unsupported binary content type: %s", mediaType))
			}
		}
	}
	return &types.ContentBlockMemberToolResult{
		Value: types.ToolResultBlock{
			ToolUseId: aws.String(toolUseID),
			Status:    types.ToolResultStatusSuccess,
			Content:   content,
		},
	}
}

func toolCall(ctx context.Context, tools estellm.ToolSet, toolUseID string, toolName string, input any) (types.ContentBlock, error) {
	for _, tool := range tools {
		if NormalizeToolName(tool.Name()) == toolName {
			w := estellm.NewBatchResponseWriter()
			if err := tool.Call(ctx, input, w); err != nil {
				return newToolResultWithError(toolUseID, err), nil
			}
			return newToolResultWithResponse(toolUseID, w.Response()), nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", toolName)
}

func mergeContentBlock(a, b types.ContentBlock) types.ContentBlock {
	if a == nil {
		return b
	}
	switch a := a.(type) {
	case *types.ContentBlockMemberText:
		if b, ok := b.(*types.ContentBlockMemberText); ok {
			return &types.ContentBlockMemberText{
				Value: a.Value + b.Value,
			}
		}
	case *types.ContentBlockMemberReasoningContent:
		if b, ok := b.(*types.ContentBlockMemberReasoningContent); ok {
			ac, ok := a.Value.(*types.ReasoningContentBlockMemberReasoningText)
			if !ok {
				return b
			}
			bc, ok := b.Value.(*types.ReasoningContentBlockMemberReasoningText)
			if !ok {
				return b
			}
			return &types.ContentBlockMemberReasoningContent{
				Value: &types.ReasoningContentBlockMemberReasoningText{
					Value: types.ReasoningTextBlock{
						Signature: coalesce(ac.Value.Signature, bc.Value.Signature),
						Text:      concat(ac.Value.Text, bc.Value.Text),
					},
				},
			}
		}
	case *types.ContentBlockMemberToolUse:
		if b, ok := b.(*types.ContentBlockMemberToolUse); ok {
			return &types.ContentBlockMemberToolUse{
				Value: types.ToolUseBlock{
					Name:      coalesce(a.Value.Name, b.Value.Name),
					ToolUseId: coalesce(a.Value.ToolUseId, b.Value.ToolUseId),
					Input:     coalesce(a.Value.Input, b.Value.Input),
				},
			}
		}
	}
	return b
}

func setToMetadata(v types.ConverseStreamMetadataEvent, m metadata.Metadata) {
	if v.Metrics != nil {
		if v.Metrics.LatencyMs != nil {
			m.SetInt64("Metrics-Latency-Ms", *v.Metrics.LatencyMs)
		}
	}
	if v.Usage != nil {
		if v.Usage.InputTokens != nil {
			m.SetInt64("Usage-Input-Tokens", int64(*v.Usage.InputTokens))
		}
		if v.Usage.OutputTokens != nil {
			m.SetInt64("Usage-Output-Tokens", int64(*v.Usage.OutputTokens))
		}
		if v.Usage.TotalTokens != nil {
			m.SetInt64("Usage-Total-Tokens", int64(*v.Usage.TotalTokens))
		}
	}
	if v.Trace != nil {
		if v.Trace.PromptRouter != nil {
			if v.Trace.PromptRouter.InvokedModelId != nil {
				m.SetString("Model-Id", *v.Trace.PromptRouter.InvokedModelId)
			}
		}
		if v.Trace.Guardrail != nil {
			if v.Trace.Guardrail.InputAssessment != nil {
				if bs, err := json.Marshal(v.Trace.Guardrail.InputAssessment); err == nil {
					m.SetString("Guardrail-Input-Assessment", string(bs))
				}
			}
			if v.Trace.Guardrail.OutputAssessments != nil {
				if bs, err := json.Marshal(v.Trace.Guardrail.OutputAssessments); err == nil {
					m.SetString("Guardrail-Output-Assessment", string(bs))
				}
			}
			if v.Trace.Guardrail.ModelOutput != nil {
				m.SetString("Guardrail-Model-Output", strings.Join(
					v.Trace.Guardrail.ModelOutput, ",",
				))
			}
		}
	}
}

func processMessageStart(_ context.Context, v *types.ConverseStreamOutputMemberMessageStart, w estellm.ResponseWriter) error {
	switch v.Value.Role {
	case types.ConversationRoleAssistant:
		w.WriteRole(estellm.RoleAssistant)
	case types.ConversationRoleUser:
		w.WriteRole(estellm.RoleUser)
	default:
		return estellm.ErrInvalidMessageRole
	}
	return nil
}

func processMessageStop(_ context.Context, v *types.ConverseStreamOutputMemberMessageStop, w estellm.ResponseWriter) (bool, error) {
	bs, err := json.Marshal(v.Value.AdditionalModelResponseFields)
	if err != nil {
		bs = []byte("{}")
	}
	switch v.Value.StopReason {
	case types.StopReasonEndTurn:
		w.Finish(estellm.FinishReasonEndTurn, string(bs))
	case types.StopReasonMaxTokens:
		w.Finish(estellm.FinishReasonMaxTokens, string(bs))
	case types.StopReasonToolUse:
		return true, nil
	case types.StopReasonStopSequence:
		w.Finish(estellm.FinishReasonStopSequence, string(bs))
	case types.StopReasonGuardrailIntervened:
		w.Finish(estellm.FinishReasonGuardrailIntervened, string(bs))
	case types.StopReasonContentFiltered:
		w.Finish(estellm.FinishReasonContentFiltered, string(bs))
	default:
		return false, fmt.Errorf("unsupported stop reason: %s", v.Value.StopReason)
	}
	return false, nil
}

func processContentBlockStart(ctx context.Context, v *types.ConverseStreamOutputMemberContentBlockStart, w estellm.ResponseWriter, toolInputBuilder *bytes.Buffer) (types.ContentBlock, error) {
	switch v := v.Value.Start.(type) {
	case *types.ContentBlockStartMemberToolUse:
		toolInputBuilder.Reset()
		return &types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				Name:      v.Value.Name,
				ToolUseId: v.Value.ToolUseId,
			},
		}, nil
	default:
		slog.WarnContext(ctx, "unsupported content block start", "type", fmt.Sprintf("%T", v))
		return nil, nil
	}
}

func processContentBlockDelta(ctx context.Context, v *types.ConverseStreamOutputMemberContentBlockDelta, w estellm.ResponseWriter, toolInputBuilder *bytes.Buffer) (types.ContentBlock, error) {
	switch v := v.Value.Delta.(type) {
	case *types.ContentBlockDeltaMemberText:
		if err := w.WritePart(estellm.TextPart(v.Value)); err != nil {
			return nil, err
		}
		return &types.ContentBlockMemberText{
			Value: v.Value,
		}, nil
	case *types.ContentBlockDeltaMemberReasoningContent:
		switch v := v.Value.(type) {
		case *types.ReasoningContentBlockDeltaMemberText:
			if err := w.WritePart(estellm.ReasoningPart(v.Value)); err != nil {
				return nil, err
			}
			return &types.ContentBlockMemberReasoningContent{
				Value: &types.ReasoningContentBlockMemberReasoningText{
					Value: types.ReasoningTextBlock{
						Text: aws.String(v.Value),
					},
				},
			}, nil
		case *types.ReasoningContentBlockDeltaMemberRedactedContent:
			return &types.ContentBlockMemberReasoningContent{
				Value: &types.ReasoningContentBlockMemberRedactedContent{
					Value: v.Value,
				},
			}, nil
		case *types.ReasoningContentBlockDeltaMemberSignature:
			return &types.ContentBlockMemberReasoningContent{
				Value: &types.ReasoningContentBlockMemberReasoningText{
					Value: types.ReasoningTextBlock{
						Signature: aws.String(v.Value),
					},
				},
			}, nil
		default:
			return nil, nil
		}
	case *types.ContentBlockDeltaMemberToolUse:
		toolInputBuilder.WriteString(*v.Value.Input)
		return &types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported content block type: %T", v)
	}
}
func (p *ModelProvider) GenerateImage(ctx context.Context, req *estellm.GenerateImageRequest, w estellm.ResponseWriter) error {
	if err := p.initClient(); err != nil {
		return err
	}
	// Implement the method
	return estellm.ErrModelNotFound
}

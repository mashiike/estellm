package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mashiike/estellm"
	"github.com/sashabaranov/go-openai"
)

func init() {
	// Register the provider
	estellm.RegisterModelProvider("openai", &ModelProvider{})
}

type OpenAIClient interface {
	CreateImage(ctx context.Context, request openai.ImageRequest) (openai.ImageResponse, error)
	// 他の必要なメソッドをここに追加
}

type ModelProvider struct {
	init    sync.Once
	client  OpenAIClient
	initErr error
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
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			p.initErr = errors.New("missing OPENAI_API_KEY")
			return
		}
		p.client = openai.NewClient(apiKey)
	})
	return p.initErr
}

func (p *ModelProvider) newClient(modelParams map[string]any) (OpenAIClient, error) {
	if apiKey, ok := modelParams["api_key"].(string); ok && apiKey != "" {
		return openai.NewClient(apiKey), nil
	}
	if err := p.initClient(); err != nil {
		return nil, err
	}
	return p.client, nil
}

func (p *ModelProvider) GenerateText(ctx context.Context, req *estellm.GenerateTextRequest, w estellm.ResponseWriter) error {
	// GenerateTextの実装をここに追加
	return estellm.ErrModelNotFound
}

func remarshalJSON(v1, v2 any) error {
	b1, err := json.Marshal(v1)
	if err != nil {
		return fmt.Errorf("marshal v1: %w", err)
	}
	if err := json.Unmarshal(b1, v2); err != nil {
		return fmt.Errorf("unmarshal v2: %w", err)
	}
	return nil
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
	if err := remarshalJSON(req.ModelParams, &imageReq); err != nil {
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

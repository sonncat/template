package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/propagation"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Close закрывает свободные HTTP-соединения клиента
func (c *Client) Close() {
	c.client.CloseIdleConnections()
}

type pushRequest struct {
	RequestID   string            `json:"request_id"`
	RecipientID uuid.UUID         `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data"`
}

// RequestPosition отправляет устройству водителя одну команду на получение координаты
func (c *Client) RequestPosition(
	ctx context.Context,
	tripID uuid.UUID,
	driverID uuid.UUID,
) error {
	body, err := json.Marshal(pushRequest{
		RequestID:   uuid.NewString(),
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data: map[string]string{
			"trip_id": tripID.String(),
		},
	})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/push",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(request.Header))

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("send push request: %w", err)
	}
	defer response.Body.Close()

	// Дочитываем ответ, чтобы соединение могло использоваться повторно
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return fmt.Errorf("read push response: %w", err)
	}

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("push service returned status %d", response.StatusCode)
	}

	return nil
}

package multimodal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const maxPDFTextBytes = 8 << 20

type pdfTextBuffer struct{ bytes.Buffer }

func (b *pdfTextBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxPDFTextBytes {
		return 0, fmt.Errorf("PDF extracted text exceeds limit")
	}
	return b.Buffer.Write(p)
}

// Keep the complete deterministic text layer. Scans must use a document OCR
// adapter; they must never be sent as an image_url to an image-only VLM.
func extractPDFText(ctx context.Context, raw []byte) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(raw)
	var out pdfTextBuffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("native PDF text extraction: %w", err)
	}
	text := strings.TrimSpace(out.String())
	if len([]rune(text)) < 40 {
		return "", fmt.Errorf("PDF has no usable text layer; document OCR required")
	}
	return text, nil
}

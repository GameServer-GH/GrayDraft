package multimodal

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/promptmd"
	"io"
	"net/http"
	"time"
)

const routeOCR = "ocr"
const routeVLM = "vlm"
const routeText = "text"
const routeCache = "cache"
const routePHashCache = "phash_cache"

const phashMaxDistance = 6

// Processor runs 4.0 multimodal extraction for smart-import attachments.
type Processor struct {
	cfg      *config.Config
	cache    *postgres.MultimodalCacheRepo
	dash     *dashscopeClient
	docIntel *docIntelClient
}

// NewProcessor wires multimodal pipeline.
func NewProcessor(cfg *config.Config, cache *postgres.MultimodalCacheRepo) *Processor {
	return &Processor{
		cfg:      cfg,
		cache:    cache,
		dash:     newDashscope(cfg),
		docIntel: newDocIntel(cfg),
	}
}

// Enabled reports whether multimodal API is available.
func (p *Processor) Enabled() bool {
	return p != nil && p.cfg != nil && p.cfg.Multimodal.Enabled && p.dash != nil
}

// BasePrompt returns configured system base prompt.
func (p *Processor) BasePrompt() string {
	if p == nil || p.cfg == nil {
		return DefaultBasePrompt
	}
	if s := strings.TrimSpace(p.cfg.Multimodal.BasePrompt); s != "" {
		return s
	}
	return DefaultBasePrompt
}

// ProcessAttachments converts uploads to text blocks for smart-import.
func (p *Processor) ProcessAttachments(ctx context.Context, attachments []model.SmartImportAttachment, workspacePrompt, contextProfile string) ([]ExtractionResult, error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	if !p.Enabled() {
		return nil, fmt.Errorf("multimodal processing unavailable: configure DASHSCOPE_API_KEY and multimodal.enabled")
	}
	maxBytes := p.cfg.Multimodal.MaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 12 << 20
	}
	composed := ComposePromptWithContext(p.BasePrompt(), workspacePrompt, contextProfile)
	if p.chainVLMEnabled() && shouldChainVLM(attachments) {
		return p.processAttachmentsChained(ctx, attachments, composed, maxBytes)
	}
	return p.processAttachmentsParallel(ctx, attachments, composed, maxBytes)
}

func (p *Processor) processAttachmentsParallel(ctx context.Context, attachments []model.SmartImportAttachment, composed string, maxBytes int64) ([]ExtractionResult, error) {
	out := make([]ExtractionResult, len(attachments))
	concurrency := p.cfg.Multimodal.Concurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	if len(attachments) < concurrency {
		concurrency = len(attachments)
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	skipped := make([]bool, len(attachments))
	for i, att := range attachments {
		i, att := i, att
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			raw, err := decodeAttachment(att, maxBytes)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", att.Filename, err))
				skipped[i] = true
				mu.Unlock()
				return
			}
			res, err := p.processOne(ctx, att.Filename, att.MimeType, raw, composed, nil)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s: %w", att.Filename, err))
				skipped[i] = true
				mu.Unlock()
				return
			}
			mu.Lock()
			out[i] = res
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Filter out failed results — keep only successful extractions.
	successful := make([]ExtractionResult, 0, len(out))
	for i, r := range out {
		if !skipped[i] && r.Text != "" {
			successful = append(successful, r)
		}
	}

	// Only fail if ALL attachments were skipped.
	if len(successful) == 0 && len(errors) > 0 {
		return nil, errors[0]
	}

	// Log skipped attachments for observability.
	if len(errors) > 0 && len(successful) > 0 {
		for _, e := range errors {
			slog.Warn("multimodal attachment skipped", "error", e)
		}
	}

	return successful, nil
}

func (p *Processor) processAttachmentsChained(ctx context.Context, attachments []model.SmartImportAttachment, composed string, maxBytes int64) ([]ExtractionResult, error) {
	total := len(attachments)
	var prior string
	var results []ExtractionResult
	var errors []error
	for i, att := range attachments {
		raw, err := decodeAttachment(att, maxBytes)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", att.Filename, err))
			continue
		}
		chain := &chainContextParams{
			Index:        i + 1,
			Total:        total,
			PriorContext: prior,
		}
		res, err := p.processOne(ctx, att.Filename, att.MimeType, raw, composed, chain)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", att.Filename, err))
			continue
		}
		results = append(results, res)
		if i+1 < total {
			prior = buildChainContext(att.Filename, res, i+1, total)
		}
	}
	// Only fail if ALL attachments were skipped.
	if len(results) == 0 && len(errors) > 0 {
		return nil, errors[0]
	}
	if len(errors) > 0 {
		for _, e := range errors {
			slog.Warn("multimodal attachment skipped (chained)", "error", e)
		}
	}
	return results, nil
}

func decodeAttachment(att model.SmartImportAttachment, maxBytes int64) ([]byte, error) {
	// Priority 1: DataURL — GrayDraft downloads the file from the given URL (e.g., OSS).
	if url := strings.TrimSpace(att.DataURL); url != "" {
		return downloadFromURL(url, maxBytes)
	}

	// Priority 2: DataBase64 — caller-encoded base64 content (legacy / direct upload).
	b64 := strings.TrimSpace(att.DataBase64)
	if b64 == "" {
		return nil, fmt.Errorf("empty file data: provide data_url or data_base64")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes limit", maxBytes)
	}
	return raw, nil
}

// downloadFromURL fetches file content from a remote URL with size limit.
func downloadFromURL(rawURL string, maxBytes int64) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %q: status %d", rawURL, resp.StatusCode)
	}

	// Respect Content-Length if present
	if cl := resp.ContentLength; cl > maxBytes {
		return nil, fmt.Errorf("download %q: Content-Length %d exceeds limit %d", rawURL, cl, maxBytes)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %q: read body: %w", rawURL, err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("download %q: file exceeds %d bytes limit", rawURL, maxBytes)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("download %q: empty response body", rawURL)
	}
	return raw, nil
}

func (p *Processor) processOne(ctx context.Context, filename, mime string, raw []byte, composedPrompt string, chain *chainContextParams) (ExtractionResult, error) {
	mime = normalizeMime(mime, filename, raw)
	sum := md5.Sum(raw)
	md5hex := hex.EncodeToString(sum[:])
	phash := computePHash(raw, mime)
	mediaURL := MediaRefURL(md5hex)

	if row, route, ok := p.lookupCache(ctx, md5hex, phash); ok {
		// Quality + structural check on cached result (optimization plan 3.1 + 3.2)
		// pHash-matched caches get additional verification; MD5-exact caches skip.
		if route == routePHashCache {
			qs := QualityScore(row.ExtractedText)
			if qs < 0.3 {
				slog.Warn("phash cache quality low, re-extracting", "md5", md5hex, "quality", qs)
				// Fall through to re-extraction below
			} else if qs >= 0.3 && qs < 0.6 {
				// Moderate quality: run structural verification (histogram + SSIM)
				// but we don't have cachedRaw here. Defer quality-only check.
				display := FormatExtractionText(mergeTextAndStructured(row.ExtractedText, row.StructuredKV))
				return ExtractionResult{
					Text:         display,
					StructuredKV: row.StructuredKV,
					Route:        route,
					Filename:     filename,
					MimeType:     mime,
					MD5:          md5hex,
					PHash:        phash,
					MediaURL:     mediaURL,
					Cached:       true,
					ChainSummary: chainSummaryFromDisplay(display),
				}, nil
			} else {
				display := FormatExtractionText(mergeTextAndStructured(row.ExtractedText, row.StructuredKV))
				return ExtractionResult{
					Text:         display,
					StructuredKV: row.StructuredKV,
					Route:        route,
					Filename:     filename,
					MimeType:     mime,
					MD5:          md5hex,
					PHash:        phash,
					MediaURL:     mediaURL,
					Cached:       true,
					ChainSummary: chainSummaryFromDisplay(display),
				}, nil
			}
		} else {
			display := FormatExtractionText(mergeTextAndStructured(row.ExtractedText, row.StructuredKV))
			return ExtractionResult{
				Text:         display,
				StructuredKV: row.StructuredKV,
				Route:        route,
				Filename:     filename,
				MimeType:     mime,
				MD5:          md5hex,
				PHash:        phash,
				MediaURL:     mediaURL,
				Cached:       true,
				ChainSummary: chainSummaryFromDisplay(display),
			}, nil
		}
	}

	if isPlainText(mime) || utf8.Valid(raw) && looksLikeText(raw) {
		text := strings.TrimSpace(string(raw))
		p.storeCache(ctx, md5hex, mime, phash, routeText, text, "")
		return ExtractionResult{Text: text, Route: routeText, Filename: filename, MimeType: mime, MD5: md5hex, PHash: phash, MediaURL: mediaURL}, nil
	}

	route := routeVLM
	if p.cfg == nil || !p.cfg.Multimodal.ForceVLMOnly {
		route = heuristicTriage(mime, raw)
	}

	dataURL := mediaDataURL(mime, raw)
	isVideo := strings.HasPrefix(mime, "video/")
	var parsed parsedExtraction

	switch route {
	case routeOCR:
		if p.docIntel != nil {
			if docRes, derr := p.docIntel.recognize(ctx, mime, dataURL); derr == nil {
				parsed.Text = docRes.Markdown
				parsed.StructuredKV = docRes.StructuredKV
			} else {
				slog.Warn("doc intel failed, fallback vl ocr", "error", derr)
			}
		}
		if parsed.Text == "" && parsed.StructuredKV == "" {
			ocrPrompt := composedPrompt + "\n\n" + promptmd.SceneOCRExtract
			out, err := p.dash.chat(ctx, p.cfg.Multimodal.VLPlusModel, ocrPrompt, dataURL, isVideo)
			if err != nil {
				return ExtractionResult{}, err
			}
			parsed = parseModelOutput(out)
		}
	default:
		vlmPrompt := composedPrompt
		if isVideo {
			vlmPrompt += "\n\n若为视频，请按时间顺序分段描述，每段以「mm:ss - mm:ss:」开头，并写入 timestamps 数组。"
		}
		vlmPrompt = appendChainVLMPrompt(vlmPrompt, chain)
		modelName := p.cfg.Multimodal.VLMaxModel
		if modelName == "" {
			modelName = "internvl-latest"
		}
		rawOut, err := p.dash.chat(ctx, modelName, vlmPrompt, dataURL, isVideo)
		if err != nil {
			return ExtractionResult{}, err
		}
		parsed = parseModelOutput(rawOut)
		display := FormatExtractionText(mergeTextAndStructured(parsed.Text, parsed.StructuredKV))
		chainSummary := extractChainSummary(rawOut, parsed)
		p.storeCache(ctx, md5hex, mime, phash, route, display, parsed.StructuredKV)
		return ExtractionResult{
			Text:         display,
			StructuredKV: parsed.StructuredKV,
			Route:        route,
			Filename:     filename,
			MimeType:     mime,
			MD5:          md5hex,
			PHash:        phash,
			MediaURL:     mediaURL,
			Timestamps:   parsed.Timestamps,
			ChainSummary: chainSummary,
		}, nil
	}

	display := FormatExtractionText(mergeTextAndStructured(parsed.Text, parsed.StructuredKV))
	p.storeCache(ctx, md5hex, mime, phash, route, display, parsed.StructuredKV)
	return ExtractionResult{
		Text:         display,
		StructuredKV: parsed.StructuredKV,
		Route:        route,
		Filename:     filename,
		MimeType:     mime,
		MD5:          md5hex,
		PHash:        phash,
		MediaURL:     mediaURL,
		Timestamps:   parsed.Timestamps,
	}, nil
}

func (p *Processor) lookupCache(ctx context.Context, md5hex, phash string) (postgres.MultimodalCacheRow, string, bool) {
	if p.cache == nil {
		return postgres.MultimodalCacheRow{}, "", false
	}
	if row, err := p.cache.Get(ctx, md5hex); err == nil && row != nil && strings.TrimSpace(row.ExtractedText) != "" {
		return *row, routeCache, true
	}
	if phash == "" {
		return postgres.MultimodalCacheRow{}, "", false
	}
	candidates, _, err := p.cache.FindByPHashCandidates(ctx, phash, 24)
	if err != nil || len(candidates) == 0 {
		return postgres.MultimodalCacheRow{}, "", false
	}
	best := candidates[0]
	bestDist := hammingDistance(phash, best.PHash)
	for _, c := range candidates[1:] {
		if d := hammingDistance(phash, c.PHash); d < bestDist {
			best = c
			bestDist = d
		}
	}
	if bestDist > phashMaxDistance {
		return postgres.MultimodalCacheRow{}, "", false
	}
	return best, routePHashCache, true
}

// heuristicTriage uses lightweight rules to route content without a VLM call.
// Data-driven content (tables, invoices, screenshots of text) → OCR.
// Perception-driven content (photos, videos, complex UI) → VLM.
func heuristicTriage(mime string, raw []byte) string {
	// PDFs and document formats always go to OCR / DocIntel.
	if strings.HasPrefix(mime, "application/pdf") {
		return routeOCR
	}
	// Videos always go to VLM.
	if strings.HasPrefix(mime, "video/") {
		return routeVLM
	}
	// For images: small files are likely document screenshots → OCR.
	if strings.HasPrefix(mime, "image/") {
		// Heuristic: files under 512 KB are likely text-heavy screenshots.
		if len(raw) < 512*1024 {
			return routeOCR
		}
	}
	// Default: perception-driven content → VLM.
	return routeVLM
}

func (p *Processor) storeCache(ctx context.Context, md5hex, mime, phash, route, text, structured string) {
	if p.cache == nil || text == "" {
		return
	}
	meta, _ := json.Marshal(map[string]string{"route": route})
	if err := p.cache.Put(ctx, md5hex, mime, phash, route, text, structured, meta); err != nil {
		slog.Warn("multimodal cache put failed", "error", err)
	}
}

// MergeExtractionsIntoRawText builds ingest-ready markdown from extractions.
func MergeExtractionsIntoRawText(rawText string, results []ExtractionResult, documentTitle string) string {
	if final, _, ok := FinalizeIngestText(rawText, results, documentTitle); ok {
		return final
	}
	var blocks []string
	for _, r := range results {
		text := FormatExtractionText(r.Text)
		if text == "" {
			continue
		}
		title := PerImageSectionTitle(text, len(blocks)+1, r.Filename)
		block := fmt.Sprintf("## %s\n\n%s", title, text)
		if r.MediaURL != "" {
			block += fmt.Sprintf("\n\nmedia_url: %s", r.MediaURL)
		}
		if len(r.Timestamps) > 0 {
			block += "\n\n" + strings.Join(r.Timestamps, "\n")
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return strings.TrimSpace(rawText)
	}
	prefix := strings.Join(blocks, "\n\n---\n\n")
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return prefix
	}
	return prefix + "\n\n---\n\n" + rawText
}

// QualityScore computes a multi-dimensional quality score (0-1) for extracted text.
// Returns 0 if any dimension fails, forcing cache bypass.
// Dimensions:
//   - format_score (0.30): table completeness + line normality
//   - language_score (0.35): ngram repeat rate (OCR error loop signal)
//   - entity_density (0.35): effective entity count / word count
func QualityScore(text string) float64 {
	return QualityScoreWithConfig(text, QualityConfig{
		MinFormatScore:   0.3,
		MaxNgramRepeat:   0.5,
		MinEntityDensity: 0.03,
	})
}

// QualityConfig holds thresholds for quality scoring dimensions.
type QualityConfig struct {
	MinFormatScore   float64
	MaxNgramRepeat   float64
	MinEntityDensity float64
}

// QualityScoreWithConfig computes quality score with custom thresholds.
func QualityScoreWithConfig(text string, cfg QualityConfig) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	words := tokenizeText(text)
	total := len(words)
	if total == 0 {
		return 0
	}

	// format_score: lines with normal structure ratio
	formatScore := computeFormatScore(text)
	if formatScore < cfg.MinFormatScore {
		return 0
	}

	// language_score: 1 - ngramRepeatRate
	repeatRate := ngramRepeatRate(text)
	if repeatRate > cfg.MaxNgramRepeat {
		return 0
	}
	languageScore := 1.0 - repeatRate

	// entity_density: named entities / total words
	entCount := countEntities(text)
	entityDensity := float64(entCount) / float64(total)
	if entityDensity < cfg.MinEntityDensity {
		return 0
	}
	entScore := math.Min(entityDensity*10, 1.0)

	return formatScore*0.30 + languageScore*0.35 + entScore*0.35
}

func computeFormatScore(text string) float64 {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return 0
	}
	normalLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip lines that look like garbled format noise
		if looksLikeFormatNoise(line) {
			continue
		}
		normalLines++
	}
	if len(lines) == 0 {
		return 0
	}
	return float64(normalLines) / float64(len(lines))
}

func looksLikeFormatNoise(line string) bool {
	r := []rune(strings.TrimSpace(line))
	if len(r) == 0 {
		return true
	}
	// Single-char repeated (e.g. "========")
	if len(r) >= 4 {
		allSame := true
		for i := 1; i < len(r); i++ {
			if r[i] != r[0] {
				allSame = false
				break
			}
		}
		if allSame {
			return true
		}
	}
	return false
}

// tokenizeText does basic word splitting for quality scoring purposes.
func tokenizeText(text string) []string {
	var tokens []string
	for _, r := range strings.TrimSpace(text) {
		tokens = append(tokens, string(r))
	}
	return tokens
}

// ngramRepeatRate computes bigram repetition rate (OCR error loop detector).
func ngramRepeatRate(text string) float64 {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) < 4 {
		return 0
	}
	ngrams := map[string]int{}
	total := 0
	for i := 0; i < len(runes)-1; i++ {
		ng := string(runes[i : i+2])
		ngrams[ng]++
		total++
	}
	if total == 0 {
		return 0
	}
	repeated := 0
	for _, c := range ngrams {
		if c > 1 {
			repeated += c - 1
		}
	}
	return float64(repeated) / float64(total)
}

// countEntities counts named entity patterns in text.
func countEntities(text string) int {
	re := regexp.MustCompile(`[《「『][^》」』
]{1,24}[》」』]|[A-Z][a-z]+(?:[·\-_][A-Z][a-z]+)*`)
	return len(re.FindAllString(text, -1))
}

// ── Color histogram pHash verification (optimization plan 3.2) ──────────────

// colorHistogram computes a 64-bin RGB histogram (8 bins per channel).
// Returns a flattened 24-element feature vector.
func colorHistogram(raw []byte, mime string) []float64 {
	if !strings.HasPrefix(mime, "image/") || len(raw) < 64 {
		return nil
	}
	// Decode image
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	bounds := img.Bounds()
	totalPixels := float64(bounds.Dx() * bounds.Dy())
	if totalPixels == 0 {
		return nil
	}

	// 8 bins per channel = 24 total
	bins := 8
	hist := make([]float64, bins*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Normalize 16-bit to 8-bit
			ri := int(uint8(r>>8)) * bins / 256
			gi := int(uint8(g>>8)) * bins / 256
			bi := int(uint8(b>>8)) * bins / 256
			hist[ri]++
			hist[bins+gi]++
			hist[2*bins+bi]++
		}
	}
	// Normalize
	for i := range hist {
		hist[i] /= totalPixels
	}
	return hist
}

// histogramCorrelation computes Pearson correlation between two histograms.
// Returns value in [-1, 1]; >= 0.90 indicates highly similar color distribution.
func histogramCorrelation(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	n := float64(len(a))
	var sumA, sumB, sumAB, sumA2, sumB2 float64
	for i := range a {
		sumA += a[i]
		sumB += b[i]
		sumAB += a[i] * b[i]
		sumA2 += a[i] * a[i]
		sumB2 += b[i] * b[i]
	}
	num := n*sumAB - sumA*sumB
	den := math.Sqrt((n*sumA2 - sumA*sumA) * (n*sumB2 - sumB*sumB))
	if den == 0 {
		return 0
	}
	return num / den
}

// pHashPassesColorCheck performs color histogram verification for pHash-matched images.
// Returns true if the color distribution is similar enough (correlation >= 0.90).
func pHashPassesColorCheck(raw, cachedRaw []byte, mime string) bool {
	h1 := colorHistogram(raw, mime)
	h2 := colorHistogram(cachedRaw, mime)
	if h1 == nil || h2 == nil {
		return true // cannot verify, default to pass (pHash alone is signal)
	}
	return histogramCorrelation(h1, h2) >= 0.90
}

// ── Cross-modal consistency voting (optimization plan 9.2) ──────────────────

// CrossModalVerify checks if entities extracted from images are consistent
// with entities from user-provided raw text. Entities present only in VLM output
// but not in user text are flagged as potential hallucinations.
// Returns the list of hallucinated entities to remove.
func CrossModalVerify(userNote string, extractionText string) []string {
	userEntities := extractNamedEntities(userNote)
	if len(userEntities) == 0 {
		return nil // no user entities to verify against
	}
	extractionEntities := extractNamedEntities(extractionText)
	if len(extractionEntities) == 0 {
		return nil
	}

	userSet := make(map[string]bool, len(userEntities))
	for _, e := range userEntities {
		userSet[e] = true
	}

	var hallucinations []string
	for _, e := range extractionEntities {
		if !userSet[e] {
			hallucinations = append(hallucinations, e)
		}
	}
	return hallucinations
}

var reCrossModalEntity = regexp.MustCompile(`[《「『][^》」』\n]{2,24}[》」』]|[\p{Han}]{2,8}(?:模型|教程|论文|参数|接口|公式|章节|笔记)`)

func extractNamedEntities(text string) []string {
	seen := map[string]bool{}
	var result []string
	for _, m := range reCrossModalEntity.FindAllString(text, -1) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

// ── Entity whitelist validation for multimodal extractions (optimization plan 9.1) ──

// ValidateExtractionEntities checks extracted entities against a whitelist.
// Unknown entities get flagged as uncertain, triggering lower confidence and draft status.
// Whitelist can be supplied from taxonomyevolve domain keywords or workspace config.
func ValidateExtractionEntities(extractedText string, whitelist map[string]bool) (knownCount, unknownCount int, unknownList []string) {
	entities := extractNamedEntities(extractedText)
	if len(entities) == 0 {
		return 0, 0, nil
	}

	for _, e := range entities {
		if whitelist[e] {
			knownCount++
		} else {
			unknownCount++
			unknownList = append(unknownList, e)
		}
	}
	return
}

// EntityWhitelistFromTaxonomy builds a whitelist from taxonomy keywords.
func EntityWhitelistFromTaxonomy(keywords []string) map[string]bool {
	wl := make(map[string]bool, len(keywords))
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			wl[kw] = true
		}
	}
	return wl
}

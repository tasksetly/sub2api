package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

// fingerprintRefModelsCacheTTL 是参考库模型清单的内存缓存时长。
// 清单只用于前端下拉，参考库重训频率以天计，短缓存足够。
const fingerprintRefModelsCacheTTL = 30 * time.Minute

// fingerprintBundleFetchTimeout 限制拉取 bundle.json 的时间。
const fingerprintBundleFetchTimeout = 20 * time.Second

// FingerprintDetectorProvider 持有懒加载的 detector 单例。
//
// 加载要解析约 4.5 MB 检测参数（Go 约 31ms），而一次评分只要 2.3ms，
// 所以 detector 必须常驻复用，绝不能每次检测重新加载。
type FingerprintDetectorProvider struct {
	settingSvc *SettingService

	mu       sync.Mutex
	detector *lmfpd.Detector
	// loadedWith 是产生当前 detector 的配置指纹；配置变了要重新加载。
	loadedWith string

	modelsMu       sync.Mutex
	models         []FingerprintRefModel
	modelsFetched  time.Time
	modelsSourceID string

	httpClient *http.Client
}

// NewFingerprintDetectorProvider 创建 detector 提供者。
func NewFingerprintDetectorProvider(settingSvc *SettingService) *FingerprintDetectorProvider {
	return &FingerprintDetectorProvider{
		settingSvc: settingSvc,
		httpClient: &http.Client{Timeout: fingerprintBundleFetchTimeout},
	}
}

// Get 返回常驻的 detector，必要时加载一次。
//
// 加载失败不缓存失败状态，下次调用会重试，并把原始错误原样返回——
// 上层要能在界面上显示"参考数据加载失败"的具体原因，而不是一句笼统的检测失败。
func (p *FingerprintDetectorProvider) Get(ctx context.Context) (*lmfpd.Detector, error) {
	if p == nil {
		return nil, fmt.Errorf("fingerprint detector provider is unavailable")
	}

	settings := p.settings(ctx)
	fingerprint := fingerprintLoadFingerprint(settings)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.detector != nil && p.loadedWith == fingerprint {
		return p.detector, nil
	}

	detector, err := lmfpd.Load(ctx, buildFingerprintLoadOptions(settings))
	if err != nil {
		return nil, fmt.Errorf("加载指纹参考数据失败: %w", err)
	}

	p.detector = detector
	p.loadedWith = fingerprint
	return detector, nil
}

// BuiltAt 返回当前参考库的生成时间，用于界面上显示数据新鲜度。
func (p *FingerprintDetectorProvider) BuiltAt(ctx context.Context) (string, error) {
	detector, err := p.Get(ctx)
	if err != nil {
		return "", err
	}
	return detector.BuiltAt(), nil
}

// ListReferenceModels 返回参考库里的全部模型，供前端的预期模型下拉使用。
//
// 预期模型只能从这个清单里选：参考库是封闭集合，库里没有的模型会被硬配一个
// "最像"的候选，放任自由输入会导致每一轮都判不符的永久误报。
func (p *FingerprintDetectorProvider) ListReferenceModels(ctx context.Context) ([]FingerprintRefModel, error) {
	if p == nil {
		return nil, fmt.Errorf("fingerprint detector provider is unavailable")
	}

	settings := p.settings(ctx)
	sourceID := strings.Join(settings.DataSources, "|")

	p.modelsMu.Lock()
	defer p.modelsMu.Unlock()

	if p.models != nil && p.modelsSourceID == sourceID && time.Since(p.modelsFetched) < fingerprintRefModelsCacheTTL {
		return append([]FingerprintRefModel(nil), p.models...), nil
	}

	raw, err := p.fetchBundle(ctx, settings)
	if err != nil {
		return nil, err
	}
	models, err := parseFingerprintBundleModels(raw)
	if err != nil {
		return nil, err
	}

	p.models = models
	p.modelsFetched = time.Now()
	p.modelsSourceID = sourceID
	return append([]FingerprintRefModel(nil), models...), nil
}

// fetchBundle 按顺序尝试每个来源，取第一个成功的。
func (p *FingerprintDetectorProvider) fetchBundle(ctx context.Context, settings FingerprintSettings) ([]byte, error) {
	if isFingerprintLocalDirectorySource(settings.DataSources) {
		path := filepath.Join(settings.DataSources[0], "unified_bank.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取本地参考库 %s 失败: %w", path, err)
		}
		return raw, nil
	}

	var lastErr error
	for _, source := range settings.DataSources {
		if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
			continue
		}
		raw, err := p.fetchBundleFrom(ctx, source)
		if err != nil {
			lastErr = err
			continue
		}
		return raw, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的参考数据源")
	}
	return nil, fmt.Errorf("拉取参考库清单失败: %w", lastErr)
}

func (p *FingerprintDetectorProvider) fetchBundleFrom(ctx context.Context, source string) ([]byte, error) {
	url := strings.TrimSuffix(source, "/") + "/bundle.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s 返回 HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func (p *FingerprintDetectorProvider) settings(ctx context.Context) FingerprintSettings {
	if p.settingSvc == nil {
		return FingerprintSettings{
			DataSources:       append([]string(nil), FingerprintDefaultDataSources...),
			CacheTTL:          FingerprintDefaultCacheTTLHours * time.Hour,
			MaxConcurrency:    FingerprintDefaultMaxConcurrency,
			DailyLimitGlobal:  FingerprintDefaultDailyLimitGlobal,
			DailyLimitAccount: FingerprintDefaultDailyLimitAccount,
		}
	}
	return p.settingSvc.GetFingerprintSettings(ctx)
}

// buildFingerprintLoadOptions 把配置翻译成 SDK 的加载参数。
func buildFingerprintLoadOptions(settings FingerprintSettings) lmfpd.LoadOptions {
	opts := lmfpd.LoadOptions{
		Sources: append([]string(nil), settings.DataSources...),
		TTL:     settings.CacheTTL,
	}
	// 本地目录来源时 SDK 不联网也不读写缓存，传 Cache 没有意义。
	if !isFingerprintLocalDirectorySource(settings.DataSources) {
		opts.Cache = settings.CachePath
	}
	return opts
}

// isFingerprintLocalDirectorySource 判断来源是否为"单个本地目录"。
// SDK 只有在恰好一个来源且它不以 http(s) 开头时才按本地目录读取。
func isFingerprintLocalDirectorySource(sources []string) bool {
	if len(sources) != 1 {
		return false
	}
	s := sources[0]
	return s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")
}

// fingerprintLoadFingerprint 是配置的指纹，用于判断是否需要重新加载 detector。
func fingerprintLoadFingerprint(settings FingerprintSettings) string {
	return strings.Join(settings.DataSources, "|") + "#" + settings.CachePath + "#" + settings.CacheTTL.String()
}

// fingerprintBundle 是 bundle.json 里我们关心的部分。
type fingerprintBundle struct {
	BuiltAt string                `json:"built_at"`
	Models  []FingerprintRefModel `json:"models"`
}

// parseFingerprintBundleModels 解析参考库清单，按家族名再按 ID 排序。
func parseFingerprintBundleModels(raw []byte) ([]FingerprintRefModel, error) {
	var bundle fingerprintBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, fmt.Errorf("解析参考库清单失败: %w", err)
	}

	models := make([]FingerprintRefModel, 0, len(bundle.Models))
	for _, m := range bundle.Models {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		if m.DisplayName == "" {
			m.DisplayName = m.ID
		}
		models = append(models, m)
	}

	sort.SliceStable(models, func(i, j int) bool {
		if models[i].FamilyName != models[j].FamilyName {
			return models[i].FamilyName < models[j].FamilyName
		}
		return models[i].ID < models[j].ID
	})
	return models, nil
}

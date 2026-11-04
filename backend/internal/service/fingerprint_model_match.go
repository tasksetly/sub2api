package service

import (
	"regexp"
	"strings"
)

// 上游常见的模型名后缀，匹配参考库前先剥掉。
// 中转商给的名字常是 claude-opus-5-latest 或 claude-opus-5-20260101，
// 而参考库里是规范化的 claude-opus-5。
var (
	fingerprintDateSuffix    = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$|-\d{8}$|-\d{6}$`)
	fingerprintAliasSuffixes = []string{"-latest", "-preview", "-stable", "-online"}
)

// ResolveFingerprintExpectedModel 把上游的模型名对应到参考库里的规范 ID。
//
// 管理员只选一个模型：发给上游的名字和用来比对的参考库 ID 本就是同一个东西，
// 只是命名空间不同。这里负责把前者翻译成后者。
//
// 匹配顺序，严格程度递减：
//  1. 完全相等
//  2. 剥掉日期/别名后缀后相等
//  3. 参考库 ID 是剥掉后缀后名字的前缀，取最长的那个
//
// 第 3 步用最长前缀，是为了让 claude-opus-5.5-xxx 命中 claude-opus-5.5
// 而不是误配到 claude-opus-5。匹配不上就返回 false——参考库是封闭集合，
// 硬配一个"最像"的候选会让这条计划每一轮都判不符。
func ResolveFingerprintExpectedModel(testModel string, known []FingerprintRefModel) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(testModel))
	if name == "" || len(known) == 0 {
		return "", false
	}

	for _, m := range known {
		if strings.ToLower(m.ID) == name {
			return m.ID, true
		}
	}

	stripped := stripFingerprintModelSuffixes(name)
	if stripped != name {
		for _, m := range known {
			if strings.ToLower(m.ID) == stripped {
				return m.ID, true
			}
		}
	}

	best := ""
	for _, m := range known {
		id := strings.ToLower(m.ID)
		if !strings.HasPrefix(stripped, id) {
			continue
		}
		// 必须在分隔处截断，避免 gpt-5 命中 gpt-54。
		if len(stripped) > len(id) {
			switch stripped[len(id)] {
			case '-', '_', '.', ':', '@':
			default:
				continue
			}
		}
		if len(id) > len(best) {
			best = m.ID
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

// stripFingerprintModelSuffixes 剥掉日期与别名后缀，可重复剥离。
func stripFingerprintModelSuffixes(name string) string {
	for {
		next := fingerprintDateSuffix.ReplaceAllString(name, "")
		for _, suffix := range fingerprintAliasSuffixes {
			next = strings.TrimSuffix(next, suffix)
		}
		if next == name {
			return name
		}
		name = next
	}
}

// fingerprintVersionToken 匹配模型 ID 里的版本片段（数字与小数点）。
var fingerprintVersionToken = regexp.MustCompile(`[0-9.]+`)

// FingerprintModelsEquivalent 判断实判模型与预期模型是否算"对得上"。
//
// 实判结果不必和预期模型逐字相同：去掉版本号后同一条产品线就算通过。
// 这样 claude-opus-5 与 claude-opus-5.5、deepseek-v4-flash 与 deepseek-v4.1-flash
// 都不会误报，而 opus 被换成 haiku、claude 被换成 deepseek 仍然判不符
// ——降级冒充正是这个功能要抓的东西，不能一起放过。
func FingerprintModelsEquivalent(expected, predicted string, known []FingerprintRefModel) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	predicted = strings.ToLower(strings.TrimSpace(predicted))
	if expected == "" || predicted == "" {
		return false
	}
	if expected == predicted {
		return true
	}

	// 家族不同一律不算对上，避免跨家族因命名巧合被放行。
	expectedFamily, okA := fingerprintFamilyOf(expected, known)
	predictedFamily, okB := fingerprintFamilyOf(predicted, known)
	if okA && okB && expectedFamily != predictedFamily {
		return false
	}

	return fingerprintModelBase(expected) == fingerprintModelBase(predicted)
}

// fingerprintModelBase 去掉版本片段，留下产品线标识。
func fingerprintModelBase(id string) string {
	return fingerprintVersionToken.ReplaceAllString(id, "")
}

// fingerprintFamilyOf 从参考库里查模型所属家族。
func fingerprintFamilyOf(id string, known []FingerprintRefModel) (string, bool) {
	for _, m := range known {
		if strings.ToLower(m.ID) == id {
			return strings.ToLower(m.Family), true
		}
	}
	return "", false
}

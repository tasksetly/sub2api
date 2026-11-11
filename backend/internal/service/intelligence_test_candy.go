package service

import "regexp"

const intelligenceTestCandyPrompt = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

| 形状 | 苹果味 | 桃子味 | 西瓜味 |
| 圆形 | 7 | 9 | 8 |
| 五角星形 | 7 | 6 | 4 |`

var candyNumberPattern = regexp.MustCompile(`[+-]?(?:[0-9]+(?:[.,][0-9]+)*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`)

// Match a standalone integer, not a digit inside another number or identifier.
func candyAnswerPasses(answer string) bool {
	for _, span := range candyNumberPattern.FindAllStringIndex(answer, -1) {
		if answer[span[0]:span[1]] != "21" {
			continue
		}
		if span[0] > 0 && candyIdentifierByte(answer[span[0]-1]) {
			continue
		}
		if span[1] < len(answer) && candyIdentifierByte(answer[span[1]]) {
			continue
		}
		return true
	}
	return false
}

func candyIdentifierByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

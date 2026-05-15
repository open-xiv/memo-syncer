package memo

var cnWorlds = map[string]struct{}{
	"红玉海": {}, "神意之地": {}, "拉诺西亚": {}, "幻影群岛": {},
	"萌芽池": {}, "宇宙和音": {}, "沃仙曦染": {}, "晨曦王座": {},
	"紫水栈桥": {}, "摩杜纳": {}, "静语庄园": {}, "延夏": {},
	"海猫茶屋": {}, "柔风海湾": {}, "琥珀原": {},
	"白金幻象": {}, "旅人栈桥": {}, "拂晓之间": {}, "龙巢神殿": {},
	"潮风亭": {}, "神拳痕": {}, "白银乡": {}, "梦羽宝境": {},
	"太阳海岸": {}, "银泪湖": {}, "伊修加德": {}, "水晶塔": {},
	"亚马乌罗提": {}, "红茶川": {},
}

func IsCNServer(s string) bool {
	_, ok := cnWorlds[s]
	return ok
}

func CNWorldList() []string {
	out := make([]string, 0, len(cnWorlds))
	for w := range cnWorlds {
		out = append(out, w)
	}
	return out
}

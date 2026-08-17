package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost 是 bcrypt 的计算成本参数：实际迭代次数为 2^bcryptCost 次。
// 10 是业界通用的「安全性与速度」平衡点；越大越慢（越抗暴力破解），但登录也越慢。
const bcryptCost = 10

// HashPassword 用 bcrypt 对明文密码做「加盐哈希」，返回可直接入库的哈希字符串。
//
// 关键点：bcrypt 每次都会自动生成一个随机盐，并把盐一起编码进结果字符串里，
// 所以即便两个用户密码完全相同，存进库里的哈希也不一样 —— 这就让「彩虹表」
// （预先算好常见密码的哈希对照表）彻底失效，因为攻击者无法预先知道盐。
//
// 返回的哈希形如 $2a$10$<22字符盐><31字符哈希>，盐和成本都内嵌其中，校验时无需单独存盐。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword 校验明文密码与已存哈希是否匹配。
// 匹配返回 true；密码错误或哈希格式非法返回 false。
func VerifyPassword(plain, hash string) bool {
	// CompareHashAndPassword 会从 hash 里读出盐和成本，用同样的参数重算一次再比对。
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

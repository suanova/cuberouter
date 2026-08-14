package setting

var AlipayAppId = ""
var AlipayPrivateKey = ""        // 应用私钥 RSA2
var AlipayPublicKey = ""         // 支付宝公钥
var AlipayNotifyUrl = ""         // 回调地址（站点根域名，为空时自动拼接 ServerAddress 作为前缀）
var AlipayMinTopUp = 1           // 最低充值数量（单位取决于额度展示类型）
var AlipaySandboxEnabled = false // 沙箱模式开关

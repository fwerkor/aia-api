# AIA API 管理员：原生渠道额外设置

本设置继承自底层兼容渠道，用于 **管理员手动配置的原生渠道**，不是 `/shared-pool` 用户贡献密钥的自定义端点配置。共享端点有额外的 HTTPS、防 SSRF 与验证约束，不能通过此处绕过。

支持的常用 JSON 选项：

| 字段 | 类型 | 作用 |
| --- | --- | --- |
| `force_format` | boolean | 按原生渠道适配器规则强制格式化 OpenAI 兼容输出 |
| `thinking_to_content` | boolean | 将 `reasoning_content` 整理到 `<think>` 标记 |
| `proxy` | string | 原生渠道的出站代理 URL，支持 `http`、`https`、`socks5`、`socks5h` |

```json
{
  "force_format": true,
  "thinking_to_content": false,
  "proxy": "socks5://proxy.example:1080"
}
```

代理 URL 保存时必须含协议和主机，不得含 query/fragment，path 只能为空或 `/`；SOCKS 默认端口为 1080。历史数据可能含无效后缀，旧配置在运行时会忽略后缀并产生脱敏警告，但再次编辑保存时需修正。拨号/KeepAlive 超时均为 30 秒，TLS 握手超时 10 秒。

普通用户使用 AIA API 不需要配置这些参数；详见 [用户指南](../usage.md) 和 [共享池实现](../../AIA-SHARED-POOL.md)。

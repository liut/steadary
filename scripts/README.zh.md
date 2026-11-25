# 脚本

| 脚本 | 说明 |
|------|------|
| `steadary-mcp.ts` | MCP 客户端 (STDIO) |
| `test-mcp.ts` | MCP HTTP 端点测试 |
| `check-env.sh` | 检查系统依赖 |
| `test-api.sh` | HTTP REST API 测试 |
| `test-grpc.sh` | gRPC API 测试 |

## 使用

```bash
# 检查环境
./scripts/check-env.sh

# 测试 HTTP API
./scripts/test-api.sh
STEADARY_ADDR=192.168.1.100:9000 ./scripts/test-api.sh

# 测试 gRPC
./scripts/test-grpc.sh 192.168.1.100:2280

# MCP 客户端
STEADARY_API=http://localhost:2280 npx tsx scripts/steadary-mcp.ts
STEADARY_UID=alice npx tsx scripts/steadary-mcp.ts
```

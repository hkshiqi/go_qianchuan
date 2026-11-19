# 千川运营 MCP

工作台负责登录授权、账户上下文、素材保护与可视化；MCP 负责为代理提供稳定、可审计的只读工具。当前部署默认关闭全部投放写操作。

## 部署地址

- 工作台：<https://dkl4ii67.zjz-service.cn/>
- 健康检查：<https://dkl4ii67.zjz-service.cn/health>
- 服务端工具 API：`https://dkl4ii67.zjz-service.cn/api/mcp`
- 标准 MCP stdio：服务器 `/opt/go-qianchuan/backend/bin/qianchuan-mcp`

## 只读工具

| Tool | Purpose |
| --- | --- |
| `qianchuan_get_status` | 账户上下文与读写开关 |
| `qianchuan_list_plans` | 计划列表、消耗与成交 |
| `qianchuan_get_plan` | 单计划详情 |
| `qianchuan_list_plan_materials` | 素材指标与当日消耗前三保护名单 |
| `qianchuan_material_report` | 素材报表、点击率、转化率与 GPM |
| `qianchuan_live_overview` | 当日直播间投放概览 |

素材删除保护由服务端执行：拉取全部分页，按服务端当日 `stat_cost_for_roi2` 显式降序排序，锁定前三名；有消耗素材所在状态为 `ALL`，避免暂停素材漏保护。删除接口无法通过客户端日期缩小保护窗口，也无法仅凭标题删除。

## MCP 客户端配置

可在 MCP 客户端中使用通用 stdio 配置。远端客户端通过 SSH 启动包装器；包装器从服务器受保护的私有环境文件读取 `MCP_API_KEY`，密钥不进入客户端参数、仓库或日志。

```json
{
  "mcpServers": {
    "qianchuan": {
      "command": "ssh",
      "args": [
        "-p",
        "<SSH_PORT>",
        "<DEPLOY_USER>@<DEPLOY_HOST>",
        "/opt/go-qianchuan/backend/bin/qianchuan-mcp.sh"
      ]
    }
  }
}
```

生产 SSH 需要在客户端本机 `~/.ssh/config` 中配置私钥、端口和主机别名，或通过 MCP 客户端支持的 SSH 认证方式提供私钥。

包装器默认读取 `/opt/go-qianchuan/backend/.env`。若客户端运行在服务器本机，可把 `command` 直接指向 `/opt/go-qianchuan/backend/bin/qianchuan-mcp.sh`；也可由启动环境提供：

```text
MCP_API_KEY=<server-private-key>
QIANCHUAN_MCP_BASE_URL=http://127.0.0.1:18081/api/mcp
```

## 授权

工具 API 不会从网页 Cookie 读取授权。首次使用需要打开工作台完成千川 OAuth，绑定账户：

```text
advertiser_id=1855810750424170
aweme_id=63307887035
live_room_id=7709635958206991154
```

授权完成后，Token Manager 会把可刷新凭证用 AES-256-GCM 加密写入 `/opt/go-qianchuan/data/tokens.json`，文件权限固定为 `600`。文件只保存 nonce 与 ciphertext，不保存明文 access token 或 refresh token。

## 写操作边界

`OPERATIONS_WRITE_ENABLED=false` 时：

- MCP 写接口统一返回 HTTP `403`。
- 工作台除只读 POST 报表接口外，统一被 `WriteGuard` 拦截。
- 创建计划、改预算、改 ROI、暂停、加素材、删素材均不可执行。

恢复投放前，仍需人工明确解除暂停；本篇配置不构成自动恢复开关。

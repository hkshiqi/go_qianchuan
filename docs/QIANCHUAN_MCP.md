# 千川运营 MCP

工作台负责登录授权、账户上下文、素材保护与可视化；MCP 负责为代理提供稳定、可审计的只读工具。当前部署默认关闭全部投放写操作。

## 部署地址

- 工作台：<https://dkl4ii67.zjz-service.cn/>
- 健康检查：<https://dkl4ii67.zjz-service.cn/health>
- 服务端 MCP：`https://dkl4ii67.zjz-service.cn/mcp`
- 标准 MCP stdio：服务器 `/opt/go-qianchuan/backend/bin/qianchuan-mcp`

## 只读工具

| Tool | Purpose |
| --- | --- |
| `qianchuan_get_status` | 账户上下文与读写开关 |
| `qianchuan_list_plans` | 计划列表、消耗与成交 |
| `qianchuan_get_plan` | 单计划详情 |
| `qianchuan_get_plan_materials` | 素材指标与当日消耗前三保护名单 |
| `qianchuan_material_report` | 素材报表、点击率、转化率与 GPM |
| `qianchuan_live_overview` | 当日直播间投放概览 |

素材删除保护由服务端执行：拉取全部分页，按服务端当日 `stat_cost_for_roi2` 显式降序排序，锁定前三名；有消耗素材所在状态为 `ALL`，避免暂停素材漏保护。删除接口无法通过客户端日期缩小保护窗口，也无法仅凭标题删除。

## MCP 客户端配置

所有使用人共用同一个公网地址，但各自在自己的 MCP 客户端配置中保存独立 Bearer Key。服务端只根据 `Authorization: Bearer <key>` 解析调用人，租户和广告主不能由工具参数指定。

优先使用远程 HTTP MCP：

```text
URL: https://dkl4ii67.zjz-service.cn/mcp
Header: Authorization: Bearer <调用人自己的租户 Key>
```

不同 MCP 客户端填写请求头的字段名不同；只要该客户端支持远程 HTTP MCP，就填写上述 URL 和 Authorization 请求头。Key 必须保存在调用人自己的客户端 Secret/环境变量中，不能写入共享配置文件、共享服务器目录或 MCP 工具参数。

仅当客户端不支持远程 HTTP MCP 时，才在本机运行 stdio 代理：

```json
{
  "mcpServers": {
    "qianchuan": {
      "command": "<本机 qianchuan-mcp 路径>",
      "env": {
        "QIANCHUAN_TENANT_KEY": "<调用人自己的租户 Key>",
        "QIANCHUAN_MCP_BASE_URL": "https://dkl4ii67.zjz-service.cn/mcp"
      }
    }
  }
}
```

包装器不再接受 `TENANT_ID` 参数，也不会从服务器共享目录按租户名读取 Key。每个使用人只在自己的进程中提供 `QIANCHUAN_TENANT_KEY`，知道别人的租户 ID 或广告主 ID 不能切换账户。

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

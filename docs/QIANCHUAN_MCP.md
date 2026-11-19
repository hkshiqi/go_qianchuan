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
| `qianchuan_list_accounts` | 当前 Key 可切换的投放账户与逐账户授权状态 |
| `qianchuan_list_plans` | 计划列表、消耗与成交 |
| `qianchuan_get_plan` | 单计划详情 |
| `qianchuan_get_plan_materials` | 素材指标与当日消耗前三保护名单 |
| `qianchuan_material_report` | 素材报表、点击率、转化率与 GPM |
| `qianchuan_live_overview` | 当日直播间投放概览 |

素材删除保护由服务端执行：拉取全部分页，按服务端当日 `stat_cost_for_roi2` 显式降序排序，锁定前三名；有消耗素材所在状态为 `ALL`，避免暂停素材漏保护。删除接口无法通过客户端日期缩小保护窗口，也无法仅凭标题删除。

## MCP 客户端配置

所有使用人共用同一个公网地址，但各自在自己的 MCP 客户端配置中保存独立 Bearer Key。服务端只根据 `Authorization: Bearer <key>` 解析调用人的租户权限域。一个 Key 可以绑定多个投放账户，账户类工具通过 `account_id` 逐请求选择；不能传 `advertiser_id` 或 `tenant_id` 绕过权限域。

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

包装器不再接受 `TENANT_ID` 参数，也不会从服务器共享目录按租户名读取 Key。每个使用人只在自己的进程中提供 `QIANCHUAN_TENANT_KEY`，知道别人的租户 ID、广告主 ID 或 `account_id` 不能切换账户。

## 多账户切换

先执行：

```text
qianchuan_list_accounts()
```

返回每账户的 `account_id`、`advertiser_id`、`aweme_id`、`live_room_id`、`authenticated` 和独立 `authorization_url`。之后所有账户类工具都传入选中的 `account_id`：

```json
{
  "account_id": "123456789012345",
  "ad_id": "987654321012345"
}
```

单账户租户可省略 `account_id`。多账户租户未传时调用直接返回错误，不会隐式使用第一个账户；因此同一个 MCP 服务并发处理多个工具调用也不会串账户。

## 授权

工具 API 不会从网页 Cookie 读取授权。首次使用应调用 `qianchuan_list_accounts`，逐个打开各账户返回的 OAuth 链接。指定账户的授权 state 只写入该账户的 Token。旧版单账户 OAuth 链接继续兼容。

示例账户：

```text
advertiser_id=1855810750424170
aweme_id=63307887035
live_room_id=7709635958206991154
```

授权完成后，Token Manager 会把可刷新凭证用 AES-256-GCM 加密写入该租户独占的 `/opt/go-qianchuan/data/tenants/<tenant>/tokens.json`，目录权限固定为 `700`，文件权限固定为 `600`。文件只保存 nonce 与 ciphertext，不保存明文 access token 或 refresh token。不同租户不会读取或覆盖彼此的凭证文件。

## 租户管理

管理员在服务器运行 `qianchuan-tenant`。创建时生成一次性显示的随机 Key，只保存其 HMAC-SHA256 摘要：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant create \
  --id operator-a \
  --name "运营A" \
  --advertiser-id 123456789012345 \
  --aweme-id 63307887035 \
  --live-room-id 7709635958206991154 \
  --key-file /opt/go-qianchuan/tenants/operator-a.key
```

将 Key 文件通过私密渠道交给对应调用人，然后从服务器上删除临时导入文件；服务端只需要租户存储中的 Key 摘要。撤回访问权限：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant delete --id operator-a
```

删除会同时吊销该 Key、解除广告主绑定并清理该租户的 Token 目录。查看租户绑定：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant list
```

同一运营人员管理多个广告主时，先把这些账户绑定到同一租户，再按请求传 `account_id`。同一个广告主仍只能归属一个租户，避免两个 Key 对同一账户产生冲突授权。

增加或移除账户：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant add-account --id operator-a --advertiser-id 223456789012345 --aweme-id 63307887035 --live-room-id 7709635958206991154
/opt/go-qianchuan/backend/bin/qianchuan-tenant remove-account --id operator-a --advertiser-id 223456789012345
```

## 写操作边界

`OPERATIONS_WRITE_ENABLED=false` 时：

- MCP 写接口统一返回 HTTP `403`。
- 工作台除只读 POST 报表接口外，统一被 `WriteGuard` 拦截。
- 创建计划、改预算、改 ROI、暂停、加素材、删素材均不可执行。

恢复投放前，仍需人工明确解除暂停；本篇配置不构成自动恢复开关。

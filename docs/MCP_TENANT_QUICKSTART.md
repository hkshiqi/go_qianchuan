# 千川 MCP 多账户隔离接入

所有人共用同一个 MCP 地址，但每人使用不同的 Bearer Key。Key 决定你能访问的租户权限域，该权限域下可以绑定多个投放账户。

账户切换按请求进行，不维护“当前账户”全局状态：

1. `qianchuan_list_accounts` 列出本 Key 可操作的账户。
2. 账户类工具传 `account_id` 指定本次调用的账户。
3. 只有一个账户时可以省略 `account_id`；多个账户时省略会返回 `account_selection_required`。
4. `account_id` 只能从本 Key 的账户列表中选择，其他租户的账户会被拒绝。

## 调用人配置

```json
{
  "mcpServers": {
    "qianchuan": {
      "type": "http",
      "url": "https://dkl4ii67.zjz-service.cn/mcp",
      "headers": {
        "Authorization": "Bearer <管理员私发给你的个人 Key>"
      }
    }
  }
}
```

个人 Key 只放自己的 MCP 客户端配置或 Secret Store，不放进聊天、共享文档、共享服务器目录，也不作为工具参数传递。

首次调用执行 `qianchuan_list_accounts`。每个账户分别返回授权状态和独立 OAuth 链接；多账户需要逐个授权。随后调用 `qianchuan_get_status` 并传 `account_id` 核对选中账户的 `advertiser_id`、`aweme_id`、`live_room_id` 与授权状态。

示例：

```json
{
  "name": "qianchuan_live_overview",
  "arguments": {
    "account_id": "123456789012345"
  }
}
```

## 管理员开通

```bash
sudo -u root /opt/go-qianchuan/backend/bin/qianchuan-tenant create \
  --id operator-a \
  --name "运营A" \
  --advertiser-id 123456789012345 \
  --aweme-id 63307887035 \
  --live-room-id 7709635958206991154 \
  --key-file /opt/go-qianchuan/tenants/operator-a.key
```

创建后只把 `operator-a.key` 私发给对应调用人。一个广告主只能绑定一个租户，避免两个 Key 对同一账户看到不同授权状态。

给同一个 Key 增加账户：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant add-account \
  --id operator-a \
  --advertiser-id 223456789012345 \
  --aweme-id 63307887035 \
  --live-room-id 7709635958206991154
```

移除账户：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant remove-account \
  --id operator-a \
  --advertiser-id 223456789012345
```

也可以创建时用 `--accounts-file` 一次绑定多个账户。文件是 JSON 数组：

```json
[
  {
    "advertiser_id": 223456789012345,
    "aweme_id": 63307887035,
    "live_room_id": 7709635958206991154
  }
]
```

撤销：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant delete --id operator-a
```

该操作吊销 Key、释放广告主绑定、清理该租户的 Token 目录。查询绑定：

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant list
```

## 隔离规则

- 工具参数中的 `advertiser_id`、`advertiserId`、`tenant_id`、`tenantId`、`api_key`、`apiKey` 会被递归拒绝。
- `account_id` 是唯一允许的账户选择参数，服务端会校验它属于当前 Bearer Key。
- 每个租户的 OAuth Token 独立保存为 `/opt/go-qianchuan/data/tenants/<tenant>/tokens.json`。
- OAuth state 同时绑定租户和所选账户、10 分钟过期且只能使用一次。
- 指定账户的 OAuth 回调只保存该账户的 Token；授权返回结果中其他账户不会被顺带写入。
- 删除租户后，即使以同名和同一广告主重建，服务端也会按新绑定重建运行态，不复用旧账户上下文。
- 写操作当前关闭；隔离机制不会自动开启投放。

## 生产验收

```bash
/opt/go-qianchuan/scripts/verify_mcp_tenant_isolation.sh
```

脚本检查健康状态、无 Key 返回 `401`、旧入口返回 `404`、owner 账户绑定、写入关闭，并用独立临时进程验证两个租户 Key 不会互串、一个 Key 可以列出并选择两个账户、跨租户 `account_id` 被拒绝。临时租户、Key、Token 和测试端口会在退出时清理。

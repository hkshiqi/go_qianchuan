# 千川 MCP 多账户隔离接入

所有人共用同一个 MCP 地址，但每人或每个广告主体使用不同的 Bearer Key。账户上下文由 Key 唯一决定，不能通过工具参数切换。

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

首次调用执行 `qianchuan_get_status`。返回的 `tenant_id`、`advertiser_id`、`aweme_id`、`live_room_id` 应该是你自己的绑定；`authenticated=false` 时打开返回的 `authorization_url` 完成千川 OAuth。

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
- 每个租户的 OAuth Token 独立保存为 `/opt/go-qianchuan/data/tenants/<tenant>/tokens.json`。
- OAuth state 绑定租户、10 分钟过期且只能使用一次。
- 授权返回的广告主必须与租户绑定一致，否则拒绝保存。
- 删除租户后，即使以同名和同一广告主重建，服务端也会按新绑定重建运行态，不复用旧账户上下文。
- 写操作当前关闭；隔离机制不会自动开启投放。

## 生产验收

```bash
/opt/go-qianchuan/scripts/verify_mcp_tenant_isolation.sh
```

脚本检查健康状态、无 Key 返回 `401`、旧入口返回 `404`、owner 账户绑定、写入关闭，并用独立临时进程验证两个租户 Key 不会互串。临时租户、Key、Token 和测试端口会在退出时清理。

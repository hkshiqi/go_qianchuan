# dkl4ii67.zjz-service.cn 部署说明

## 当前拓扑

- 公网工作台：`https://dkl4ii67.zjz-service.cn/qianchuan-app/`
- Nginx：监听宿主机 `8080`，静态站点为 `/opt/go-qianchuan/frontend/dist`
- Go 后端：systemd `qianchuan-backend.service`，监听 `127.0.0.1:18081`
- MCP 公网入口：`https://dkl4ii67.zjz-service.cn/qianchuan-app/mcp`
- MCP 租户隔离：每个使用人一把独立 Bearer Key，可绑定多个投放账户；调用时用 `account_id` 选择
- 访问控制：`/access` 页面使用 `APP_ACCESS_USERNAME` 和 `APP_ACCESS_PASSWORD_SHA256`，Session 保存在服务端内存
- OAuth 回调需要跨站携带访问 Session，因此生产环境使用 `COOKIE_SECURE=true`、`COOKIE_SAME_SITE=none`
- Nginx 将 `/api/`、`/health`、`/ready`、`/metrics` 和 `/swagger/` 转发到 Go 后端
- 旧 `qc-workbench.service` 保留在 `/opt/qc-live-workbench`，切换后已停止并禁用开机自启

## 首次部署

```bash
cd /opt/go-qianchuan/backend
export PATH=/opt/toolchains/go1.24.5/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
go build -trimpath -o bin/qianchuan-server ./cmd/server
go build -trimpath -o bin/qianchuan-mcp ./cmd/qianchuan-mcp
go build -trimpath -o bin/qianchuan-tenant ./cmd/qianchuan-tenant

install -m 0755 deploy/zjz-service/qianchuan-mcp.sh /opt/go-qianchuan/backend/bin/qianchuan-mcp.sh

install -m 0644 deploy/zjz-service/qianchuan-backend.service /etc/systemd/system/qianchuan-backend.service
install -m 0644 deploy/zjz-service/go-qianchuan.conf /www/server/panel/vhost/nginx/go-qianchuan.conf
systemctl daemon-reload
systemctl enable --now qianchuan-backend.service
nginx -t
nginx -s reload
```

## 创建 MCP 租户

```bash
install -d -m 0700 /opt/go-qianchuan/tenants
/opt/go-qianchuan/backend/bin/qianchuan-tenant create \
  --id owner \
  --name "运营A" \
  --advertiser-id 1855810750424170 \
  --aweme-id 63307887035 \
  --live-room-id 7710006340290513690 \
  --key-file /opt/go-qianchuan/tenants/owner.key
chmod 600 /opt/go-qianchuan/tenants/owner.key
```

广告主只能绑定一个租户。同一个人管理多个广告主时，把多个账户加到同一租户，用 `account_id` 逐请求切换；不能通过 MCP 参数指定 `advertiser_id` 或 `tenant_id`。

```bash
/opt/go-qianchuan/backend/bin/qianchuan-tenant add-account \
  --id owner \
  --advertiser-id 223456789012345 \
  --aweme-id 63307887035 \
  --live-room-id 7710006340290513690
```

多账户客户端先调用 `qianchuan_list_accounts`，随后给账户类工具传选中的 `account_id`。多账户未传会拒绝执行，不会默认第一个账户。OAuth 链接按账户生成，每个账户需分别授权；一次授权只写所选账户的 Token。

## MCP 客户端

所有使用人共用 `https://dkl4ii67.zjz-service.cn/qianchuan-app/mcp`，但分别持有自己的 Bearer Key。Key 只配置在各调用人的 MCP 客户端 Secret 或本机环境中，不能放进共享目录或工具参数。

```text
URL: https://dkl4ii67.zjz-service.cn/qianchuan-app/mcp
Header: Authorization: Bearer <调用人自己的租户 Key>
```

仅当客户端不支持远程 HTTP MCP 时，才在本机运行 stdio 代理，并通过本机环境提供 `QIANCHUAN_TENANT_KEY` 和 `QIANCHUAN_MCP_BASE_URL`。`qianchuan-mcp.sh` 不接受租户参数，也不会按租户名读取服务器共享 Key 文件，避免共用 SSH 账户的人切换账户。

## 回滚到旧工作台

```bash
systemctl stop qianchuan-backend.service
systemctl disable qianchuan-backend.service
rm -f /www/server/panel/vhost/nginx/go-qianchuan.conf
rm -f /www/server/panel/vhost/nginx/go-qianchuan-test.conf
nginx -t
nginx -s reload
systemctl enable --now qc-workbench.service
```

旧工作台的数据和程序均未删除。

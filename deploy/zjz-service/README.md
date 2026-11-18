# dkl4ii67.zjz-service.cn 部署说明

## 当前拓扑

- 公网：`https://dkl4ii67.zjz-service.cn/`
- Nginx：监听宿主机 `8080`，静态站点为 `/opt/go-qianchuan/frontend/dist`
- Go 后端：systemd `qianchuan-backend.service`，监听 `127.0.0.1:18081`
- 访问控制：`/access` 页面使用 `APP_ACCESS_USERNAME` 和 `APP_ACCESS_PASSWORD_SHA256`，Session 保存在服务端内存
- Nginx 将 `/api/`、`/health`、`/ready`、`/metrics` 和 `/swagger/` 转发到 Go 后端
- 旧 `qc-workbench.service` 保留在 `/opt/qc-live-workbench`，切换后已停止并禁用开机自启

## 首次部署

```bash
cd /opt/go-qianchuan/backend
export PATH=/opt/toolchains/go1.24.5/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
go build -trimpath -o bin/qianchuan-server ./cmd/server

install -m 0644 deploy/zjz-service/qianchuan-backend.service /etc/systemd/system/qianchuan-backend.service
install -m 0644 deploy/zjz-service/go-qianchuan.conf /www/server/panel/vhost/nginx/go-qianchuan.conf
systemctl daemon-reload
systemctl enable --now qianchuan-backend.service
nginx -t
nginx -s reload
```

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

# mf650高级后台

适用于已核对的 MF650 / Qualcomm SDX55 固件的设备管理后台，界面显示名称为 **5G MIFI**。

## 功能

- 深色仪表盘，蜂窝状态、CPU、内存、温度、电池及流量信息。
- 居中的用户名和密码登录，沿用设备现有管理员账号。
- 蜂窝、频段、Wi-Fi、局域网、防火墙、短信和系统管理。
- 内置卡、内置副卡、外置卡三卡切换。
- AT 命令、IMEI 管理入口、网页终端入口。
- 电池充电策略、缓存清理、双频同名设置。
- LTE 小区信息及锁定接口；5G 扫描和锁定取决于固件实际能力，不保证所有版本可用。

## ADB 部署

设备必须开启 root ADB。完整下载仓库，将官方 Android Platform Tools 放入 `platform-tools/` 或加入 PATH，双击 `一键部署.cmd`。

1. **双后台兼容**：保留原机后台，新版通过 `http://192.168.100.1:8780/` 访问。
2. **80 端口正式接管**：直接访问 `http://192.168.100.1/` 使用新版，不跳转 8780。原网页及旧 httpd 程序移入恢复备份，独立兼容接口仅监听设备回环地址。

仓库不公开分发原厂 httpd 和网页固件文件。首次部署会从你自己的兼容设备保存接口运行环境为本地 `native-runtime.tar.gz`；该文件不要提交到仓库。请勿使用不同固件机器的接口包。

第二种模式保留底层网络、nvram、ubus、短信与驱动服务，并不把整个固件重写。不能删除 `/data/airlink/native`、兼容接口服务或底层固件服务。模式 2 将原网页和原服务程序从活动位置移入回退备份；保留独立兼容接口及底层固件。模式 1 则恢复原后台与新版 8780 共存。

设备现有 IMEI、MAC、校准、密钥及个人配置不随本仓库发布。实际设置、端口、APN 和硬件能力以本机为准。本项目不是 9008 刷机包，不保证其他型号兼容。

## 构建及测试

需要 Go，目标 Linux ARMv7。仓库提供当前自编译主程序及自有 QoS 辅助程序；辅助程序源代码位于 `native/qos_observer.c`。

```sh
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o airlink-console .
```

本次发布未附带开源许可证；第三方组件权利归各自权利人。公开可见不等于自动授予任意再分发权限。

## 两种模式均执行断流修复

- 停用并屏蔽 `net_check.service`，移走 `/usr/bin/net_check.sh`：该旧检测脚本曾在探测失败时切换 SIM。
- 停用并屏蔽 `thttpd.service`，移走 `/usr/bin/thttpd`：本机该旧服务缺配置，曾反复启动失败。
- 停用并屏蔽 `webserver.service`，移走 `/usr/bin/webserver`：本机已抓到其周期性操作数据拨号，停用后周期性断流消失。

处理范围只限上述三个已诊断的旧服务，不会停止 network_mgr、qmi_svr、兼容 httpd、充电或无线驱动服务。原文件移入本次部署的 `legacy-disconnect/` 备份目录；这些修复保留生效，回退界面也不会重新启用旧断流程序。不会改 SIM、APN、频段或 Wi-Fi 配置。不能保证解决信号质量差、基站负载等其他原因造成的卡顿。

## 兼容接口回环辅助库

`native/loopback-bind.so` 仅注入兼容 httpd，将其 80 端口绑定限制到回环地址；新版绑定设备 LAN IP 的 80 端口。源码位于 `native/loopback_bind.c`。与当前程序相同，已提供 ARMv7 二进制。可使用 Zig 交叉编译：

```sh
zig cc -target arm-linux-gnueabihf.2.28 -mcpu=cortex_a7 -shared -fPIC -Os native/loopback_bind.c -o native/loopback-bind.so -ldl
```

本次已在匹配设备验证回环监听、两种模式切换与只读状态接口；未执行写入网络配置、发送短信或电源操作来验证所有功能。

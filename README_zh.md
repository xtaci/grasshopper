# 🦗 grasshopper
[![GoDoc][1]][2] [![MIT licensed][3]][4] [![Created At][5]][6] [![Release][9]][10]

[1]: https://godoc.org/github.com/xtaci/grasshopper?status.svg
[2]: https://pkg.go.dev/github.com/xtaci/grasshopper
[3]: https://img.shields.io/badge/license-MIT-blue.svg
[4]: LICENSE
[5]: https://img.shields.io/github/created-at/xtaci/grasshopper
[6]: https://img.shields.io/github/created-at/xtaci/grasshopper
[9]: https://img.shields.io/github/v/release/xtaci/grasshopper?color=orange
[10]: https://github.com/xtaci/grasshopper/releases/latest

[English](README.md) | [中文](README_zh.md)

**Grasshopper** 是一款轻量级 UDP 数据包转发器，负责监听入站数据包并将其路由到预先配置的目标节点。它能够在入站与出站链路上分别应用独立的密钥和加密算法，既可实现端到端加密，也可在各跳之间灵活切换安全策略。

## 架构
Grasshopper 以“跳点链”方式串联多个中继节点，能够在不同链路上进行重新加密或解密。以下示例展示了一个多跳 DNS 查询流程：
```
                              ┌────────────┐              ┌───────────────┐                                  
                              │ ENCRYPTED  │              │ RE-ENCRYPTION │                                  
                              └─────┬──────┘              │ AES ───► 3DES │                                  
                                    │                     └──────┬────────┘                                  
                                    │                            │                                           
                                    │                            │                                           
                                    │                            │                                           
                                    │                            │                                           
                   ┌─────────┐      │       ┌────────────┐       │           ┌─────────┐                     
                 <HOP0>      │      │      <HOP1>        │       │        <HOP2>      HOPS(FINAL)            
 ┌─────────┐       └         │      ▼       └            │       │           │       ┌─┴──┐ ┌────────────┐   
 │ dig xxx ├─► CLEAR TEXT    ┼────(AES)──────►           │       ▼           └ DNS   │DNS1├─► 8.8.8.8:53 │   
 │ @hop0   │       ┌         │              ┌            ├─────(3DES)────────► QUERY │DNS2│ └────────────┘   
 └─────────┘       │  ▲      │              │     ▲      │                   ┌       └─┬──┘                  
                   │  │      │              │     │      │                   │         │                     
                   └──┼──────┘              └─────┼──────┘                   └─────────┘                     
                      │                           │                                                          
                      │                           │                                                          
                      │                           │                                                          
                      │                           │                                                          
                   ┌──┼────────┐                  │                                                          
                   │           │                  │                                                          
                   │ OPTIONAL  ├──────────────────┘                                                          
                   │ PACKET    │                                                                             
                   │ PROCESSOR │                                                                             
                   │           │                                                                             
                   └───────────┘                                                                             
```

## 安装

使用 Go 工具链可直接获取最新发行版：

```sh
go install  github.com/xtaci/grasshopper/cmd/grasshopper@latest     
```

## 参数
运行 `grasshopper --help` 可获取完整说明，核心选项如下：

```text
Grasshopper 是一个 UDP 数据包转发器，它监听传入的数据包并将其转发到配置的目标地址。它支持对传入和传出的数据包进行加密，并且可以使用不同的密钥和加密方法。

用法:
  grasshopper [command]

可用命令:
  completion  为指定 shell 生成自动补全脚本
  help        查看任意命令的帮助信息
  start       启动 UDP 中继监听器

标志:
      --ci string          入站数据的解密算法。可选: aes, aes-128, aes-192, qpp, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (默认 "qpp")
      --co string          出站数据的加密算法。可选: aes, aes-128, aes-192, qpp, salsa20, blowfish, twofish, cast5, 3des, tea, xtea, sm4, none (默认 "qpp")
  -c, --config string      配置文件路径
  -h, --help               显示帮助
      --ki string          客户端侧（最后一跳）复用的密钥 (默认 "it's a secret")
      --ko string          下一跳使用的密钥 (默认 "it's a secret")
  -l, --listen string      监听地址，例如 "IP:1234" (默认 ":1234")
  -n, --nexthops strings   下一跳服务器列表，按哈希随机转发 (默认 [127.0.0.1:3000])
      --sockbuf int        监听套接字缓冲区大小 (默认 1048576)
      --timeout duration   UDP 连接空闲超时时间 (默认 1m0s)
  -t, --toggle             切换帮助信息
  -v, --version            输出版本号

使用 "grasshopper [command] --help" 深入了解具体命令。
```

## 加密算法支持
- SM4 ([国密](https://en.wikipedia.org/wiki/SM4_(cipher)))
- AES ([Advanced Encryption Standard](https://en.wikipedia.org/wiki/Advanced_Encryption_Standard)), 128, 192, 256-bit
- QPP ([Quantum Permutation Pad](https://epjquantumtechnology.springeropen.com/articles/10.1140/epjqt/s40507-022-00145-y))
- Salsa20 (https://en.wikipedia.org/wiki/Salsa20)
- Blowfish (https://en.wikipedia.org/wiki/Blowfish_(cipher))
- Twofish (https://en.wikipedia.org/wiki/Twofish)
- Cast5 (https://en.wikipedia.org/wiki/CAST-128)
- 3DES (https://en.wikipedia.org/wiki/Triple_DES)
- Tea ([Tiny Encryption Algorithm](https://en.wikipedia.org/wiki/Tiny_Encryption_Algorithm))
- XTea (https://en.wikipedia.org/wiki/XTEA)

## 使用案例

### 案例 I: 安全回显 (Secure Echo)

流程速览：

```
客户端 (ncat @127.0.0.1:4000)
  │ 明文
  ▼
一级中继 (ci=none, co=aes) ── AES 密文 ──► 二级中继 (ci=aes, co=none) ──► UDP echo @127.0.0.1:5000
```

读者只需记住：终端只连 `127.0.0.1:4000`，出站在一级中继被加密，到二级中继解密后再送往 echo 服务器。

#### 步骤 1: 启动 UDP Echo 服务器

借助 `ncat` 在 5000 端口启动一个 UDP echo 服务器：

```sh
ncat -e /bin/cat -k -u -l 5000
```
#### 步骤 2: 启动到 Echo 服务器的二级中继

运行以下命令启动中继节点：

```sh
./grasshopper start --ci aes --co none -l "127.0.0.1:4001" -n "127.0.0.1:5000"
```

- `--ci aes`: 按 AES 解密来自上一跳的数据包。
- `--co none`: 向 `ncat` echo 服务器转发明文。

#### 步骤 3: 启动到二级中继的一级中继

执行下列命令启动第一跳：

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "127.0.0.1:4001"
```

- `--ci none`: 入站数据为明文，直接透传。
- `--co aes`: 使用 AES 加密后再送往下一跳。

#### 步骤 4: 启动演示客户端

使用 `ncat` 发送 UDP 数据包，与多跳链路交互：

```sh
ncat -u 127.0.0.1 4000
```

### 案例 II: 安全 DNS 查询 (随机选择)
流程速览（双跳转发、随机上游）：

```
本地 dig ──► 一级中继 (ci=none, co=aes) ── AES 密文跨网 ──► 二级中继 (ci=aes, co=none) ──► DNS 池 {8.8.8.8, 1.1.1.1}
```

本地只需请求 `127.0.0.1:4000`；跨公网的那一跳保持加密，二级中继会为每次查询在上游池里随机挑选一个 DNS。
```
┌──────────── YOUR─LAPTOP ──────────────┐           ┌────────── CLOUD─SERVER ───────────┐
│                                       │           │                                   │
│                                       │           │                                   │
│ ┌───────────────────┐   ┌──────────┐  │           │ ┌──────────┐   ┌───────────────┐  │
│ │                   │   │          │  │           │ │          │   │               │  │
│ │ dig google.com    ├───► Level-1  │  │           │ │ Level-2  ├───► Google DNS:53 │  │
│ │ @127.0.0.1 -p 4000│   │ Relayer  ┼──┼ ENCRYPTED ┼─► Relayer  │   │ CloudFlare:53 │  │
│ │                   │   │          │  │    UDP    │ │          │   │               │  │
│ └───────────────────┘   └──────────┘  │           │ └──────────┘   └───────────────┘  │
│                                       │           │                                   │
│                                       │           │                                   │
└───────────────────────────────────────┘           └───────────────────────────────────┘
```
#### 步骤 1: 启动到 DNS 服务器的二级中继 (云服务器 🖥️)

```sh
./grasshopper start --ci aes --co none -l "CLOUD_PUBLIC_IP:4000" -n "8.8.8.8:53,1.1.1.1:53"
```

- `--ci aes`: 对来自一级中继的密文进行解密（`ci` = cipher-in）。
- `--co none`: 以明文形式将查询投递至上游 DNS（`co` = cipher-out）。

#### 步骤 2: 启动到二级中继的一级中继 (本地电脑 💻)

```sh
./grasshopper start --ci none --co aes -l "127.0.0.1:4000" -n "CLOUD_PUBLIC_IP:4000"
```

- `--ci none`: 本地 `dig` 查询为明文，无需解密。
- `--co aes`: 将 DNS 查询加密后发往云端。

#### 步骤 3: 使用 `dig` 查询一级中继 (本地电脑 💻)

```sh
dig google.com @127.0.0.1 -p 4000
```

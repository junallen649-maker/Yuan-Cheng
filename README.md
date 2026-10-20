# Polaris (DESK远程)

商业级现代化跨平台远程桌面控制系统。采用 Rust 核心引擎结合 Flutter 打造极简、优雅的“左侧树状导航边栏 + 右侧卡片式工作台”，支持毫秒级超低延迟串流、点对点穿透直连、强力中继与 Windows 静默自动安装。

---

## 🌟 核心特性

- **现代极简设计 (UU 风格)**：清爽科技蓝卡片式工作台，支持桌面壁纸大图预览、快捷操作栏、一键复制分享设备信息。
- **角色体系隔离 (Console & Agent)**：
  - **Console (控制端)**：全功能设备管理、快速启动锁屏/重启/任务管理器/Ctrl+Alt+Del、Wake-on-LAN 局域网唤醒。
  - **Agent (受控端)**：开机后台静默守护，支持 Windows 系统服务自启，无多余弹窗与黑框打扰。
- **极致速度与穿透**：Rust 编写的高性能音视频编码捕获（DirectX/DXGI、Wayland、ScreenCaptureKit），支持 TCP/UDP/KCP 穿透与加密通道。
- **全平台支持**：支持 Windows (x64/ARM64)、macOS、Linux 以及 Android 移动端。

---

## 🏗️ 项目架构

- **`src/`**：Rust 核心网络服务、音视频串流、IPC 与平台适配代码
- **`flutter/`**：基于 Flutter 3.24.5 的现代化跨平台 UI 界面
- **`libs/`**：底层独立 Crate 体系（包含屏幕捕获 `scrap`、键鼠控制 `enigo`、便携打包器 `portable`、剪贴板 `clipboard` 等）
- **`res/`**：Windows MSI 安装配置、应用图标与系统服务定义

---

## 💻 本地构建指南

### 1. 准备依赖
- 安装 Rust (推荐 1.75+ 工具链)
- 安装 Flutter SDK (3.24.5)
- 配置 `VCPKG_ROOT` 并安装多媒体基础依赖 (`libvpx`, `libyuv`, `opus`, `aom`)

### 2. 编译启动
```bash
# 运行构建总控脚本
python3 build.py --flutter --hwcodec
```

---

## 📄 授权与许可

Copyright © 2026 Polaris. All rights reserved.

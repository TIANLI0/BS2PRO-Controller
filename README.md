# BS2PRO-Controller

> 飞智空间站 BS2/BS2PRO 的第三方替代控制器

一个基于 Wails + Go + Next.js 开发的桌面应用，用于控制飞智空间站 BS2/BS2PRO 散热器设备，提供风扇控制、温度监控等功能。

## 功能特性

- 🎮 **设备支持**：支持飞智 BS2 和 BS2PRO 散热器
- 🌡️ **温度监控**：实时监控 CPU/GPU 温度（支持多种温度数据桥接方式）
- 💨 **风扇控制**：
  - 自动模式：根据温度自动调节风速
  - 学习控温：根据目标温度持续学习并微调曲线偏移
  - 手动模式：自定义固定风速
  - 曲线模式：自定义温度-风速曲线
- 📊 **可视化面板**：直观的温度和风速实时显示
- 🎯 **系统托盘**：支持最小化到系统托盘，后台运行
- 🚀 **开机自启**：可设置开机自动启动并最小化运行
- 🔧 **多进程架构**：GUI 和核心服务分离，稳定可靠
- 🛠️ **灯带配置**：支持灯带复杂调控，感谢群友 @Whether

## 系统架构

项目采用三进程架构：

- **GUI 进程** (`BS2PRO-Controller.exe`)：提供用户界面，使用 Wails 框架
- **核心服务** (`BS2PRO-Core.exe`)：后台运行，负责设备通信和温度监控
- **温度桥接进程** (`TempBridge.exe`)：通过 C# 程序获取系统温度数据

三个进程通过 IPC (进程间通信) 进行数据交互。

## 技术栈

### 后端
- **Go 1.25+**：主要开发语言
- **Wails v2**：跨平台桌面应用框架
- **go-hid**：HID 设备通信
- **zap**：日志记录

### 前端
- **Next.js 16**：React 框架
- **TypeScript**：类型安全
- **Tailwind CSS 4**：样式框架
- **Recharts**：图表可视化

### 温度桥接
- **C# .NET Framework 4.7.2**：温度数据桥接程序

## 开发环境要求

### 必需软件
- **Go 1.21+**：[下载地址](https://golang.org/dl/)
- **Node.js 18+**：[下载地址](https://nodejs.org/)
- **Bun**：快速的 JavaScript 运行时 [安装说明](https://bun.sh/)
- **Wails CLI**：安装命令 `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **.NET SDK 8.0+**：[下载地址](https://dotnet.microsoft.com/download)
- **go-winres**：Windows 资源工具 `go install github.com/tc-hib/go-winres@latest`

### 可选软件
- **NSIS 3.x**：用于生成安装程序 [下载地址](https://nsis.sourceforge.io/)

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/TIANLI0/BS2PRO-Controller.git
cd BS2PRO-Controller
```

### 2. 安装依赖

#### 安装 Go 依赖
```bash
go mod tidy
```

#### 安装前端依赖
```bash
cd frontend
bun install
cd ..
```

### 3. 开发模式运行

```bash
# 启动 Wails 开发模式（包含热重载）
wails dev
```

### 4. 构建生产版本

#### 构建温度桥接程序
```bash
build_bridge.bat
```

#### 构建完整应用
```bash
build.bat
```

构建完成后，可执行文件位于 `build/bin/` 目录：
- `BS2PRO-Controller.exe` - GUI 主程序
- `BS2PRO-Core.exe` - 核心服务
- `bridge/TempBridge.exe` - 温度桥接程序

安装程序位于 `build/bin/` 目录：
- `BS2PRO-Controller-amd64-installer.exe` - Windows 安装程序

## 项目结构

```
BS2PRO-Controller/
├── main.go                 # GUI 主程序入口
├── app.go                  # GUI 应用逻辑
├── wails.json             # Wails 配置文件
├── build.bat              # Windows 构建脚本
├── build_bridge.bat       # 桥接程序构建脚本
├── cmd/
│   └── core/              # 核心服务程序
│       ├── main.go        # 服务入口
│       └── app.go         # 服务逻辑
│
├── internal/              # 内部包
│   ├── autostart/         # 开机自启管理
│   ├── bridge/            # 温度桥接通信
│   ├── config/            # 配置管理
│   ├── device/            # HID 设备通信
│   ├── ipc/               # 进程间通信
│   ├── logger/            # 日志模块
│   ├── temperature/       # 温度监控
│   ├── tray/              # 系统托盘
│   ├── types/             # 类型定义
│   └── version/           # 版本信息
│
├── bridge/
│   └── TempBridge/        # C# 温度桥接程序
│       └── Program.cs     # 桥接程序源码
│
├── frontend/              # Next.js 前端
│   ├── src/
│   │   ├── app/
│   │   │   ├── components/    # React 组件
│   │   │   ├── services/      # API 服务
│   │   │   └── types/         # TypeScript 类型
│   │   └── ...
│   └── package.json
│
└── build/                 # 构建输出目录
```

## 使用说明

### 首次运行

1. 运行 `BS2PRO-Controller.exe` 启动程序
2. 程序会自动启动核心服务 `BS2PRO-Core.exe`
3. 连接你的 BS2/BS2PRO 设备（USB 连接）
4. 程序会自动检测并连接设备

### 风扇控制模式

#### 自动模式
- 根据当前温度自动调节风速
- 适合日常使用

#### 手动模式
- 设置固定的风速档位（0-9档）
- 适合特定需求场景

#### 曲线模式
- 自定义温度-风速曲线
- 可添加多个控制点
- 实现精细化的温度控制

### 温度监控

程序支持多种温度监控方式：

1. **TempBridge**：通过 C# 桥接程序获取系统温度


### 系统托盘

- 点击托盘图标打开主窗口
- 右键菜单提供快捷操作
- 支持最小化到托盘后台运行

## 配置文件

配置文件位于 `%APPDATA%\BS2PRO-Controller\config.json`

主要配置项：
```json
{
  "autoStart": false,           // 开机自启
  "minimizeToTray": true,       // 关闭时最小化到托盘
  "temperatureSource": "auto",  // 温度数据源
  "updateInterval": 1000,       // 更新间隔（毫秒）
  "fanCurve": [...],           // 风扇曲线
  "fanMode": "auto"            // 风扇模式
}
```

## 日志文件

日志文件位于 `build/bin/logs/` 目录：
- `core_YYYYMMDD.log` - 核心服务日志
- `gui_YYYYMMDD.log` - GUI 程序日志

## 常见问题

### 设备无法连接？
1. 确保 BS2/BS2PRO 设备已正确连接到电脑
2. 检查设备驱动是否正常安装
3. 尝试重新插拔设备
4. 查看日志文件排查具体错误

### 温度无法显示？
1. 检查温度数据源设置
2. 如使用 TempBridge，确保 `bridge` 目录下的文件完整
3. 如使用 AIDA64/HWiNFO，确保软件正在运行并开启了共享内存功能

### 开机自启无效？
1. 以管理员身份运行程序后重新设置
2. 检查注册表项：`HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`

## 构建说明

### 版本号管理

版本号在 `wails.json` 的 `info.productVersion` 字段中定义，构建脚本会自动读取并嵌入到程序中。

### LDFLAGS

构建时会注入版本信息：
```bash
-ldflags "-X github.com/TIANLI0/BS2PRO-Controller/internal/version.BuildVersion=版本号 -H=windowsgui"
```

### 生成安装程序

执行 `build.bat` 会自动生成 NSIS 安装程序（需要安装 NSIS）。

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 开源许可

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 作者

- **TIANLI0** - [GitHub](https://github.com/TIANLI0)
- Email: wutianli@tianli0.top

## 致谢

- [Wails](https://wails.io/) - 优秀的 Go 桌面应用框架
- [Next.js](https://nextjs.org/) - React 应用框架
- 飞智- BS2/BS2PRO 硬件设备

## 免责声明

本项目为第三方开源项目，与飞智官方无关。使用本软件产生的任何问题由用户自行承担。

---

⭐ 如果这个项目对你有帮助，请给一个 Star！

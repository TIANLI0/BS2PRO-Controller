# 温度桥接程序

## 概述

由于Go语言无法直接调用C#库，我们创建了一个C#桥接程序 `TempBridge.exe`。默认构建链会先同步 https://github.com/LibreHardwareMonitor/LibreHardwareMonitor 的最新 `master` 提交，再用仓库里的 `LibreHardwareMonitorLib` 源码构建 TempBridge。

如果本地没有上游源码目录，`TempBridge.csproj` 仍然保留 `LibreHardwareMonitorLib 0.9.6` 的 NuGet 兜底引用；但 `build_bridge.bat` 默认不会走这个兜底，而是先同步 GitHub 最新 commit。

## 构建说明

### 前提条件

- 安装 [.NET 8.0 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)
- 安装 Git（`build_bridge.bat` 会自动同步 LibreHardwareMonitor 最新提交）
- 可访问 GitHub 和 NuGet 源

### Windows 构建

```bash
# 在项目根目录运行
build_bridge.bat
```

脚本行为：

1. 将上游仓库同步到 `temp/LibreHardwareMonitor`
2. 读取该仓库当前 HEAD commit
3. 通过 `LibreHardwareMonitorLib.csproj` 源码引用构建 `TempBridge`
4. 下载 `PawnIO_setup.exe`

### Linux/Mac 构建（交叉编译）

```bash
# 在项目根目录运行
chmod +x build_bridge.sh
./build_bridge.sh
```

### 手动构建

```bash
cd bridge/TempBridge
dotnet restore TempBridge.csproj /p:UseLibreHardwareMonitorProjectReference=true /p:LibreHardwareMonitorRepoRoot=../../temp/LibreHardwareMonitor
dotnet publish TempBridge.csproj -c Release --self-contained false -o ../../build/bin/bridge /p:UseLibreHardwareMonitorProjectReference=true /p:LibreHardwareMonitorRepoRoot=../../temp/LibreHardwareMonitor
```

## 工作原理

1. Go程序调用 `TempBridge.exe`
2. 桥接程序通过 LibreHardwareMonitor 源码构建出的 `LibreHardwareMonitorLib` 读取硬件温度
3. 桥接程序以JSON格式输出温度数据
4. Go程序解析JSON数据并使用

## 直接启动排查

在命令行里直接运行 `TempBridge.exe` 时，程序会进入诊断模式，而不是命名管道模式：

```bash
cd bridge/TempBridge/bin/Release/net472
TempBridge.exe
```

诊断模式会直接在控制台输出：

- CPU/GPU/MAX 温度
- 当前是否读取成功
- 错误信息
- 已发现的温度传感器名称和读数

如果需要强制使用原有的管道模式，可传入 `--pipe` 参数。

`--pipe` 模式现在会使用固定命名管道和全局单实例互斥；如果系统里已经有一个 TempBridge 正在监听，新进程不会再启动第二个监听实例，而是直接附着到现有实例。

## 输出格式

```json
{
  "cpuTemp": 45,
  "gpuTemp": 38,
  "maxTemp": 45,
  "updateTime": 1692259200,
  "success": true,
  "error": ""
}
```

## 错误处理

如果桥接程序不可用或失败，Go程序会自动回退到原有的温度读取方法：

1. 使用 `gopsutil` 读取传感器数据
2. 通过WMI读取Windows系统温度
3. 使用 `nvidia-smi` 读取NVIDIA GPU温度

## 注意事项

- 桥接程序需要以管理员权限运行才能访问所有硬件传感器
- 首次运行可能需要一些时间来初始化硬件监控
- 如果遇到权限问题，请尝试以管理员身份运行主程序
- 运行前请确保系统已安装 `PawnIO`（未安装时 `TempBridge` 会在启动阶段直接报错并退出）

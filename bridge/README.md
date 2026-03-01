# 温度桥接程序

## 概述

由于Go语言无法直接调用C#库，我们创建了一个C#桥接程序 `TempBridge.exe`，通过 NuGet 引用 `LibreHardwareMonitorLib` 获取准确的CPU和GPU温度数据。

## 构建说明

### 前提条件

- 安装 [.NET 8.0 SDK](https://dotnet.microsoft.com/download/dotnet/8.0)
- 可访问 NuGet 源（`dotnet restore` 会自动拉取 `LibreHardwareMonitorLib`）

### Windows 构建

```bash
# 在项目根目录运行
build_bridge.bat
```

### Linux/Mac 构建（交叉编译）

```bash
# 在项目根目录运行
chmod +x build_bridge.sh
./build_bridge.sh
```

### 手动构建

```bash
cd bridge/TempBridge
dotnet restore
dotnet publish TempBridge.csproj -c Release --self-contained false -o ../../build/bin/bridge
```

## 工作原理

1. Go程序调用 `TempBridge.exe`
2. 桥接程序通过 NuGet 引入的 `LibreHardwareMonitorLib` 读取硬件温度
3. 桥接程序以JSON格式输出温度数据
4. Go程序解析JSON数据并使用

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

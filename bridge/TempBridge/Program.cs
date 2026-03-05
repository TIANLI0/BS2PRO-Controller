using System;
using System.IO;
using System.IO.Pipes;
using System.Threading;
using Newtonsoft.Json;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;

namespace TempBridge
{
    public class TemperatureData
    {
        public int CpuTemp { get; set; }
        public int GpuTemp { get; set; }
        public int MaxTemp { get; set; }
        public long UpdateTime { get; set; }
        public bool Success { get; set; }
        public string Error { get; set; }

        public TemperatureData()
        {
            Error = string.Empty;
        }
    }

    public class UpdateVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer)
        {
            computer.Traverse(this);
        }

        public void VisitHardware(IHardware hardware)
        {
            hardware.Update();
            foreach (IHardware subHardware in hardware.SubHardware)
                subHardware.Accept(this);
        }

        public void VisitSensor(ISensor sensor) { }
        public void VisitParameter(IParameter parameter) { }
    }

    public class Command
    {
        public string Type { get; set; }
        public string Data { get; set; }
    }

    public class Response
    {
        public bool Success { get; set; }
        public string Error { get; set; }
        public TemperatureData Data { get; set; }
    }

    class Program
    {
        private static Computer computer;
        private static bool running = true;
        private static readonly string PIPE_NAME = "TempBridge_" + System.Diagnostics.Process.GetCurrentProcess().Id;
        private static readonly object lockObject = new object();

        static void Main(string[] args)
        {
            try
            {
                // 初始化硬件监控
                InitializeHardwareMonitor();

                // 输出管道名称，让主程序知道如何连接
                Console.WriteLine($"PIPE:{PIPE_NAME}");
                Console.Out.Flush();

                // 启动管道服务器
                StartPipeServer();
            }
            catch (Exception ex)
            {
                Console.WriteLine($"ERROR:{ex.Message}");
                Environment.Exit(1);
            }
            finally
            {
                computer?.Close();
            }
        }

        static void InitializeHardwareMonitor()
        {
            EnsurePawnIoInstalled();

            computer = new Computer
            {
                IsCpuEnabled = true,
                IsGpuEnabled = true,
                IsMemoryEnabled = false,
                IsMotherboardEnabled = false,
                IsControllerEnabled = false,
                IsNetworkEnabled = false,
                IsStorageEnabled = false
            };

            computer.Open();
            computer.Accept(new UpdateVisitor());
        }

        static void EnsurePawnIoInstalled()
        {
            if (!PawnIo.IsInstalled)
            {
                throw new InvalidOperationException(
                    "检测到 LibreHardwareMonitor 需要 PawnIO 驱动，但系统未安装。" +
                    "请先安装 PawnIO（可从 LibreHardwareMonitor 发布包中的 PawnIO_setup.exe 安装），" +
                    "安装完成后重启程序。"
                );
            }
        }

        static void StartPipeServer()
        {
            while (running)
            {
                try
                {
                    using (var pipeServer = new NamedPipeServerStream(PIPE_NAME, PipeDirection.InOut))
                    {
                        // 等待客户端连接
                        pipeServer.WaitForConnection();

                        using (var reader = new StreamReader(pipeServer))
                        using (var writer = new StreamWriter(pipeServer))
                        {
                            while (pipeServer.IsConnected && running)
                            {
                                try
                                {
                                    string commandJson = reader.ReadLine();
                                    if (string.IsNullOrEmpty(commandJson))
                                        break;

                                    var command = JsonConvert.DeserializeObject<Command>(commandJson);
                                    var response = ProcessCommand(command);

                                    string responseJson = JsonConvert.SerializeObject(response);
                                    writer.WriteLine(responseJson);
                                    writer.Flush();

                                    if (command.Type == "Exit")
                                    {
                                        running = false;
                                        break;
                                    }
                                }
                                catch (Exception ex)
                                {
                                    var errorResponse = new Response
                                    {
                                        Success = false,
                                        Error = ex.Message
                                    };
                                    string errorJson = JsonConvert.SerializeObject(errorResponse);
                                    writer.WriteLine(errorJson);
                                    writer.Flush();
                                    break;
                                }
                            }
                        }
                    }
                }
                catch (Exception ex)
                {
                    if (running)
                    {
                        Console.WriteLine($"管道错误: {ex.Message}");
                        Thread.Sleep(1000); // 等待一秒后重试
                    }
                }
            }
        }

        static Response ProcessCommand(Command command)
        {
            try
            {
                switch (command.Type)
                {
                    case "GetTemperature":
                        return new Response
                        {
                            Success = true,
                            Data = GetTemperatureData()
                        };

                    case "Ping":
                        return new Response
                        {
                            Success = true,
                            Data = new TemperatureData { Success = true }
                        };

                    case "Exit":
                        return new Response
                        {
                            Success = true
                        };

                    default:
                        return new Response
                        {
                            Success = false,
                            Error = "未知命令类型"
                        };
                }
            }
            catch (Exception ex)
            {
                return new Response
                {
                    Success = false,
                    Error = ex.Message
                };
            }
        }

        static TemperatureData GetTemperatureData()
        {
            lock (lockObject)
            {
                var result = new TemperatureData
                {
                    UpdateTime = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
                };

                try
                {
                    computer.Accept(new UpdateVisitor());

                    int cpuTemp = 0;
                    int gpuTemp = 0;

                    foreach (IHardware hardware in computer.Hardware)
                    {
                        if (hardware.HardwareType == HardwareType.Cpu)
                        {
                            if (cpuTemp == 0)
                            {
                                cpuTemp = GetTemperatureFromHardwareTree(
                                    hardware,
                                    new[] { "Average", "Package", "Tctl", "Tdie", "Core" }
                                );
                            }
                        }
                        else if (hardware.HardwareType == HardwareType.GpuNvidia || 
                                 hardware.HardwareType == HardwareType.GpuAmd ||
                                 hardware.HardwareType == HardwareType.GpuIntel)
                        {
                            if (gpuTemp == 0)
                            {
                                gpuTemp = GetTemperatureFromHardwareTree(
                                    hardware,
                                    new[] { "Average", "GPU Core", "Core", "Edge", "Junction", "Hot Spot", "Temperature" }
                                );
                            }
                        }
                    }

                    result.CpuTemp = cpuTemp;
                    result.GpuTemp = gpuTemp;
                    result.MaxTemp = Math.Max(cpuTemp, gpuTemp);
                    if (cpuTemp == 0 && gpuTemp == 0)
                    {
                        result.Success = false;
                        result.Error = "未读取到有效的 CPU/GPU 温度（PawnIO 可能尚未就绪，请重启软件或重新安装驱动）";
                    }
                    else
                    {
                        result.Success = true;
                        result.Error = string.Empty;
                    }
                }
                catch (Exception ex)
                {
                    result.Success = false;
                    result.Error = ex.Message;
                }

                return result;
            }
        }

        static int GetTemperatureFromHardwareTree(IHardware hardware, string[] preferredSensorNames)
        {
            int temp = GetPreferredTemperatureFromSensors(hardware.Sensors, preferredSensorNames);
            if (temp > 0)
            {
                return temp;
            }

            foreach (IHardware subHardware in hardware.SubHardware)
            {
                temp = GetTemperatureFromHardwareTree(subHardware, preferredSensorNames);
                if (temp > 0)
                {
                    return temp;
                }
            }

            return 0;
        }

        static int GetPreferredTemperatureFromSensors(ISensor[] sensors, string[] preferredSensorNames)
        {
            int fallbackTemp = 0;

            foreach (ISensor sensor in sensors)
            {
                if (sensor.SensorType != SensorType.Temperature || !sensor.Value.HasValue)
                {
                    continue;
                }

                int temp = (int)Math.Round(sensor.Value.Value);
                if (temp <= 0 || temp >= 150)
                {
                    continue;
                }

                if (ContainsAnyKeyword(sensor.Name, preferredSensorNames))
                {
                    return temp;
                }

                if (fallbackTemp == 0)
                {
                    fallbackTemp = temp;
                }
            }

            return fallbackTemp;
        }

        static bool ContainsAnyKeyword(string source, string[] keywords)
        {
            if (string.IsNullOrEmpty(source) || keywords == null)
            {
                return false;
            }

            foreach (string keyword in keywords)
            {
                if (!string.IsNullOrEmpty(keyword) &&
                    source.IndexOf(keyword, StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    return true;
                }
            }

            return false;
        }
    }
}

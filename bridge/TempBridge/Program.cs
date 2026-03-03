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

                    int maxCpuTemp = 0;
                    int maxGpuTemp = 0;

                    foreach (IHardware hardware in computer.Hardware)
                    {
                        if (hardware.HardwareType == HardwareType.Cpu)
                        {
                            foreach (ISensor sensor in hardware.Sensors)
                            {
                                if (sensor.SensorType == SensorType.Temperature && sensor.Value.HasValue)
                                {
                                    int temp = (int)Math.Round(sensor.Value.Value);
                                    if (temp > 0 && temp < 150)
                                    {
                                        // 优先选择CPU Package温度
                                        if (sensor.Name.Contains("Package") || sensor.Name.Contains("CPU Package"))
                                        {
                                            maxCpuTemp = temp;
                                            break;
                                        }
                                        // 如果没有Package温度，选择最高的Core温度
                                        else if (sensor.Name.Contains("Core") && temp > maxCpuTemp)
                                        {
                                            maxCpuTemp = temp;
                                        }
                                        // 其他CPU温度传感器作为备选
                                        else if (!sensor.Name.Contains("Core") && maxCpuTemp == 0)
                                        {
                                            maxCpuTemp = temp;
                                        }
                                    }
                                }
                            }
                        }
                        else if (hardware.HardwareType == HardwareType.GpuNvidia || 
                                 hardware.HardwareType == HardwareType.GpuAmd ||
                                 hardware.HardwareType == HardwareType.GpuIntel)
                        {
                            foreach (ISensor sensor in hardware.Sensors)
                            {
                                if (sensor.SensorType == SensorType.Temperature && sensor.Value.HasValue)
                                {
                                    int temp = (int)Math.Round(sensor.Value.Value);
                                    if (temp > maxGpuTemp && temp < 150) // 合理温度范围
                                    {
                                        maxGpuTemp = temp;
                                    }
                                }
                            }
                        }
                    }

                    result.CpuTemp = maxCpuTemp;
                    result.GpuTemp = maxGpuTemp;
                    result.MaxTemp = Math.Max(maxCpuTemp, maxGpuTemp);
                    result.Success = true;
                }
                catch (Exception ex)
                {
                    result.Success = false;
                    result.Error = ex.Message;
                }

                return result;
            }
        }
    }
}

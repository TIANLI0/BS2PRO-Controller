using System;
using System.IO;
using System.IO.Pipes;
using System.Linq;
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
        private const string PipeName = "BS2PRO_TempBridge";
        private const string MutexName = @"Global\BS2PRO_TempBridge_Singleton";
        private static Computer computer;
        private static bool running = true;
        private static readonly object lockObject = new object();
        private static Mutex singleInstanceMutex;

        static void Main(string[] args)
        {
            try
            {
                if (ShouldRunDiagnosticMode(args))
                {
                    RunConsoleDiagnostics();
                    return;
                }

                // Initialize hardware monitoring
                using (var instanceHandle = AcquirePipeInstance())
                {
                    if (instanceHandle == null)
                    {
                        Console.WriteLine($"PIPE:{PipeName}|ATTACH");
                        Console.Out.Flush();
                        return;
                    }

                    InitializeHardwareMonitor();

                    // Output pipe name so the main program knows how to connect
                    Console.WriteLine($"PIPE:{PipeName}|OWNER");
                    Console.Out.Flush();

                    // Start pipe server
                    StartPipeServer();
                }
            }
            catch (Exception ex)
            {
                if (ShouldRunDiagnosticMode(args))
                {
                    Console.Error.WriteLine("TempBridge failed to start");
                    Console.Error.WriteLine($"Error: {ex.Message}");
                }
                else
                {
                    Console.WriteLine($"ERROR:{ex.Message}");
                }
                Environment.Exit(1);
            }
            finally
            {
                computer?.Close();
                if (singleInstanceMutex != null)
                {
                    singleInstanceMutex.Dispose();
                    singleInstanceMutex = null;
                }
            }
        }

        static IDisposable AcquirePipeInstance()
        {
            bool createdNew;
            singleInstanceMutex = new Mutex(false, MutexName, out createdNew);

            bool acquired = false;
            try
            {
                acquired = singleInstanceMutex.WaitOne(0, false);
            }
            catch (AbandonedMutexException)
            {
                acquired = true;
            }

            if (!acquired)
            {
                return null;
            }

            return new MutexHandle(singleInstanceMutex);
        }

        static bool ShouldRunDiagnosticMode(string[] args)
        {
            if (HasArg(args, "--pipe"))
            {
                return false;
            }

            if (HasArg(args, "--diag") || HasArg(args, "--diagnose"))
            {
                return true;
            }

            return Environment.UserInteractive && !Console.IsOutputRedirected;
        }

        static bool HasArg(string[] args, string expected)
        {
            if (args == null || args.Length == 0)
            {
                return false;
            }

            return args.Any(arg => string.Equals(arg, expected, StringComparison.OrdinalIgnoreCase));
        }

        static void RunConsoleDiagnostics()
        {
            Console.WriteLine("TempBridge Diagnostic Mode");
            Console.WriteLine($"Time: {DateTime.Now:yyyy-MM-dd HH:mm:ss}");
            Console.WriteLine();

            InitializeHardwareMonitor();

            TemperatureData data = GetTemperatureData();
            PrintTemperatureSummary(data);
            Console.WriteLine();
            PrintHardwareSnapshot();

            if (!data.Success)
            {
                Environment.Exit(1);
            }
        }

        static void PrintTemperatureSummary(TemperatureData data)
        {
            Console.WriteLine("Temperature Results");
            Console.WriteLine($"CPU: {FormatTemperature(data.CpuTemp)}");
            Console.WriteLine($"GPU: {FormatTemperature(data.GpuTemp)}");
            Console.WriteLine($"MAX: {FormatTemperature(data.MaxTemp)}");
            Console.WriteLine($"Success: {data.Success}");

            if (!string.IsNullOrEmpty(data.Error))
            {
                Console.WriteLine($"Error: {data.Error}");
            }
        }

        static string FormatTemperature(int value)
        {
            return value > 0 ? value + "°C" : "N/A";
        }

        static void PrintHardwareSnapshot()
        {
            Console.WriteLine("Temperature Sensor Snapshot");

            bool foundAny = false;
            foreach (IHardware hardware in computer.Hardware)
            {
                foundAny |= PrintHardwareSnapshotRecursive(hardware, 0);
            }

            if (!foundAny)
            {
                Console.WriteLine("- No temperature sensors found");
            }
        }

        static bool PrintHardwareSnapshotRecursive(IHardware hardware, int indentLevel)
        {
            bool wroteLine = false;
            string indent = new string(' ', indentLevel * 2);

            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType != SensorType.Temperature)
                {
                    continue;
                }

                string valueText = sensor.Value.HasValue
                    ? sensor.Value.Value.ToString("F1") + "°C"
                    : "N/A";
                Console.WriteLine(
                    string.Format(
                        "{0}- [{1}] {2} / {3}: {4}",
                        indent,
                        hardware.HardwareType,
                        hardware.Name,
                        sensor.Name,
                        valueText
                    )
                );
                wroteLine = true;
            }

            foreach (IHardware subHardware in hardware.SubHardware)
            {
                if (PrintHardwareSnapshotRecursive(subHardware, indentLevel + 1))
                {
                    wroteLine = true;
                }
            }

            return wroteLine;
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
                    "LibreHardwareMonitor requires the PawnIO driver, but it is not installed. " +
                    "Please install PawnIO first (available from PawnIO_setup.exe in the LibreHardwareMonitor release package), " +
                    "then restart the program."
                );
            }
        }

        static void StartPipeServer()
        {
            while (running)
            {
                try
                {
                    using (var pipeServer = new NamedPipeServerStream(PipeName, PipeDirection.InOut))
                    {
                        // Wait for client connection
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
                        Console.WriteLine($"Pipe error: {ex.Message}");
                        Thread.Sleep(1000); // Wait one second before retrying
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
                            Error = "Unknown command type"
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
                        result.Error = "No valid CPU/GPU temperature readings (PawnIO may not be ready yet, please restart the software or reinstall the driver)";
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

        sealed class MutexHandle : IDisposable
        {
            private Mutex mutex;

            public MutexHandle(Mutex mutex)
            {
                this.mutex = mutex;
            }

            public void Dispose()
            {
                if (mutex == null)
                {
                    return;
                }

                try
                {
                    mutex.ReleaseMutex();
                }
                catch (ApplicationException)
                {
                }
            }
        }
    }
}

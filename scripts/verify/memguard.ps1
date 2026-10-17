# Run a native Windows command inside a private, memory-limited job.
# Closing the job also terminates descendants left behind by the command.
param([ValidateRange(1, 1048576)][int]$LimitMB)

$ErrorActionPreference = 'Stop'
if ($args.Count -eq 0) {
    [Console]::Error.WriteLine('usage: memguard.ps1 <rss-limit-mb> <command> [args...]')
    exit 2
}
$commandPath = (Get-Command $args[0] -CommandType Application -ErrorAction Stop).Source
$commandArgs = [string[]]@($args | Select-Object -Skip 1)

Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

public static class AgentHarnessMemoryGuard {
    [StructLayout(LayoutKind.Sequential)]
    struct BasicLimits {
        public long ProcessTime, JobTime;
        public uint Flags;
        public UIntPtr MinimumWorkingSet, MaximumWorkingSet;
        public uint ActiveProcesses;
        public UIntPtr Affinity;
        public uint Priority, Scheduling;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct IoCounters {
        public ulong ReadOperations, WriteOperations, OtherOperations;
        public ulong ReadBytes, WriteBytes, OtherBytes;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct ExtendedLimits {
        public BasicLimits Basic;
        public IoCounters Io;
        public UIntPtr ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory;
    }
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern IntPtr CreateJobObject(IntPtr attributes, string name);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr job, int kind,
        ref ExtendedLimits limits, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool QueryInformationJobObject(IntPtr job, int kind,
        out ExtendedLimits limits, uint size, IntPtr returned);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll")]
    static extern bool CloseHandle(IntPtr handle);

    static string Quote(string value) {
        var result = new StringBuilder("\"");
        int slashes = 0;
        foreach (char ch in value) {
            if (ch == '\\') { slashes++; continue; }
            if (ch == '"') result.Append('\\', slashes * 2 + 1).Append(ch);
            else result.Append('\\', slashes).Append(ch);
            slashes = 0;
        }
        return result.Append('\\', slashes * 2).Append('"').ToString();
    }

    public static int Run(string executable, string[] arguments, ulong limitMB) {
        IntPtr job = CreateJobObject(IntPtr.Zero, null);
        if (job == IntPtr.Zero) throw new Win32Exception();
        try {
            var limits = new ExtendedLimits();
            // JOB_OBJECT_LIMIT_JOB_MEMORY | JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
            limits.Basic.Flags = 0x200 | 0x2000;
            limits.JobMemory = new UIntPtr(limitMB * 1024 * 1024);
            uint size = (uint)Marshal.SizeOf(typeof(ExtendedLimits));
            if (!SetInformationJobObject(job, 9, ref limits, size))
                throw new Win32Exception();
            var quoted = Array.ConvertAll(arguments, Quote);
            var start = new ProcessStartInfo(executable, string.Join(" ", quoted));
            start.UseShellExecute = false;
            start.CreateNoWindow = true;
            start.RedirectStandardOutput = true;
            start.RedirectStandardError = true;
            using (var process = Process.Start(start)) {
                if (!AssignProcessToJobObject(job, process.Handle)) {
                    int error = Marshal.GetLastWin32Error();
                    process.Kill();
                    process.WaitForExit();
                    throw new Win32Exception(error);
                }
                process.OutputDataReceived += (sender, line) => {
                    if (line.Data != null) Console.Out.WriteLine(line.Data);
                };
                process.ErrorDataReceived += (sender, line) => {
                    if (line.Data != null) Console.Error.WriteLine(line.Data);
                };
                process.BeginOutputReadLine();
                process.BeginErrorReadLine();
                // A finite wait avoids waiting for inherited output handles
                // held by descendants before we close their job.
                while (!process.WaitForExit(1000)) { }
                if (QueryInformationJobObject(job, 9, out limits, size, IntPtr.Zero))
                    Console.Error.WriteLine("[memguard] peak job memory: {0}MB of {1}MB ceiling",
                        limits.PeakJobMemory.ToUInt64() / (1024 * 1024), limitMB);
                CloseHandle(job);
                job = IntPtr.Zero;
                process.WaitForExit();
                return process.ExitCode;
            }
        } finally { if (job != IntPtr.Zero) CloseHandle(job); }
    }
}
'@

exit [AgentHarnessMemoryGuard]::Run($commandPath, $commandArgs, [uint64]$LimitMB)

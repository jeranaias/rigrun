//! GPU Detection Module for rigrun
//!
//! This module provides functionality to detect available GPUs on the system,
//! retrieve their specifications, and recommend appropriate AI models based on
//! available VRAM.
//!
//! # Supported GPU Types
//! - NVIDIA (via nvidia-smi)
//! - AMD (via rocm-smi)
//! - Apple Silicon (via system_profiler on macOS)
//! - Intel Arc (via intel_gpu_top)
//!
//! # Example
//! ```rust,no_run
//! use rigrun::detect::{detect_gpu, recommend_model};
//!
//! let gpu_info = detect_gpu().expect("Failed to detect GPU");
//! println!("Detected: {} with {}GB VRAM", gpu_info.name, gpu_info.vram_gb);
//!
//! let model = recommend_model(gpu_info.vram_gb);
//! println!("Recommended model: {}", model);
//! ```

use std::process::Command;

/// Represents the type of GPU detected on the system.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum GpuType {
    /// NVIDIA GPU (CUDA-capable)
    Nvidia,
    /// AMD GPU (ROCm-capable)
    Amd,
    /// Apple Silicon with integrated GPU (Metal-capable)
    AppleSilicon,
    /// Intel Arc discrete GPU
    Intel,
    /// No dedicated GPU found, CPU-only mode
    Cpu,
}

impl std::fmt::Display for GpuType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            GpuType::Nvidia => write!(f, "NVIDIA"),
            GpuType::Amd => write!(f, "AMD"),
            GpuType::AppleSilicon => write!(f, "Apple Silicon"),
            GpuType::Intel => write!(f, "Intel Arc"),
            GpuType::Cpu => write!(f, "CPU"),
        }
    }
}

/// Contains information about a detected GPU.
#[derive(Debug, Clone)]
pub struct GpuInfo {
    /// The name of the GPU (e.g., "NVIDIA RTX 4090")
    pub name: String,
    /// Available VRAM in gigabytes
    pub vram_gb: u32,
    /// Driver version if available
    pub driver: Option<String>,
    /// The type of GPU
    pub gpu_type: GpuType,
}

impl Default for GpuInfo {
    fn default() -> Self {
        Self {
            name: String::from("CPU Only"),
            vram_gb: 0,
            driver: None,
            gpu_type: GpuType::Cpu,
        }
    }
}

impl std::fmt::Display for GpuInfo {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{} ({}GB VRAM)", self.name, self.vram_gb)?;
        if let Some(ref driver) = self.driver {
            write!(f, " [Driver: {}]", driver)?;
        }
        Ok(())
    }
}

/// Error type for GPU detection operations.
#[derive(Debug)]
pub enum GpuDetectError {
    /// Command execution failed
    CommandFailed(String),
    /// Failed to parse GPU information
    ParseError(String),
    /// No GPU detected
    NoGpuFound,
}

impl std::fmt::Display for GpuDetectError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            GpuDetectError::CommandFailed(msg) => write!(f, "Command failed: {}", msg),
            GpuDetectError::ParseError(msg) => write!(f, "Parse error: {}", msg),
            GpuDetectError::NoGpuFound => write!(f, "No GPU found"),
        }
    }
}

impl std::error::Error for GpuDetectError {}

/// Result type for GPU detection operations.
pub type Result<T> = std::result::Result<T, GpuDetectError>;

/// Detects available GPUs on the system.
///
/// This function checks for various GPU types in the following order:
/// 1. NVIDIA GPUs (via nvidia-smi)
/// 2. AMD GPUs (via rocm-smi)
/// 3. Apple Silicon (via system_profiler on macOS)
/// 4. Intel Arc GPUs (via intel_gpu_top)
///
/// If no GPU is found, returns a `GpuInfo` with `GpuType::Cpu`.
///
/// # Returns
/// - `Ok(GpuInfo)` - Information about the detected GPU
/// - Falls back to CPU mode if no GPU is detected
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::detect_gpu;
///
/// match detect_gpu() {
///     Ok(info) => println!("Found GPU: {}", info),
///     Err(e) => eprintln!("Detection failed: {}", e),
/// }
/// ```
pub fn detect_gpu() -> Result<GpuInfo> {
    // Try NVIDIA first (most common for ML workloads)
    if let Some(info) = detect_nvidia() {
        return Ok(info);
    }

    // Try AMD (ROCm)
    if let Some(info) = detect_amd() {
        return Ok(info);
    }

    // Try Apple Silicon (macOS only)
    if let Some(info) = detect_apple_silicon() {
        return Ok(info);
    }

    // Try Intel Arc
    if let Some(info) = detect_intel_arc() {
        return Ok(info);
    }

    // No GPU found, fall back to CPU mode
    Ok(GpuInfo::default())
}

/// Detects NVIDIA GPUs using nvidia-smi.
///
/// Queries nvidia-smi for GPU name, memory, and driver version.
/// On Windows, tries standard nvidia-smi paths if the command isn't in PATH.
fn detect_nvidia() -> Option<GpuInfo> {
    // Try nvidia-smi command locations in order:
    // 1. From PATH (works on Linux/macOS and properly configured Windows)
    // 2. Windows System32 path
    // 3. Windows Program Files NVIDIA Corporation path
    let nvidia_smi_paths = if cfg!(target_os = "windows") {
        vec![
            "nvidia-smi",
            "C:\\Windows\\System32\\nvidia-smi.exe",
            "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
        ]
    } else {
        vec!["nvidia-smi"]
    };

    let mut output = None;
    for path in nvidia_smi_paths {
        if let Ok(out) = Command::new(path)
            .args([
                "--query-gpu=name,memory.total,driver_version",
                "--format=csv,noheader,nounits",
            ])
            .output()
        {
            if out.status.success() {
                output = Some(out);
                break;
            }
        }
    }

    let output = output?;
    let stdout = String::from_utf8_lossy(&output.stdout);

    // Handle potential Windows line ending issues
    let line = stdout.lines().next()?.trim();

    // nvidia-smi outputs CSV with ", " as delimiter
    let parts: Vec<&str> = line.split(", ").collect();

    if parts.len() < 3 {
        return None;
    }

    let name = format!("NVIDIA {}", parts[0].trim());

    // Memory is in MiB, convert to GB
    let vram_mb: f64 = parts[1].trim().parse().ok()?;
    let vram_gb = (vram_mb / 1024.0).round() as u32;

    let driver = Some(parts[2].trim().to_string());

    Some(GpuInfo {
        name,
        vram_gb,
        driver,
        gpu_type: GpuType::Nvidia,
    })
}

/// Infers VRAM size from AMD GPU model name.
///
/// Uses known GPU specifications to estimate VRAM for high-end GPUs
/// when direct detection methods fail or return incorrect values.
fn infer_amd_vram_from_model(gpu_name: &str) -> Option<u32> {
    let name_lower = gpu_name.to_lowercase();

    // RX 9000 series (RDNA 4) - 2025
    if name_lower.contains("9070 xt") {
        return Some(16);
    } else if name_lower.contains("9070") {
        return Some(12);
    }

    // RX 8000 series (RDNA 4)
    if name_lower.contains("8800 xt") || name_lower.contains("8900 xt") {
        return Some(16);
    } else if name_lower.contains("8700 xt") || name_lower.contains("8800") {
        return Some(12);
    }

    // RX 7000 series (RDNA 3)
    if name_lower.contains("7900 xtx") {
        return Some(24);
    } else if name_lower.contains("7900 xt") || name_lower.contains("7900 gre") {
        return Some(20);
    } else if name_lower.contains("7800 xt") {
        return Some(16);
    } else if name_lower.contains("7700 xt") {
        return Some(12);
    } else if name_lower.contains("7600 xt") {
        return Some(16);
    } else if name_lower.contains("7600") {
        return Some(8);
    }

    // Pro series (Workstation) - check first to avoid consumer card pattern collisions
    if name_lower.contains("pro w7900") {
        return Some(48);
    } else if name_lower.contains("pro w7800") {
        return Some(32);
    } else if name_lower.contains("pro w7700") {
        return Some(16);
    } else if name_lower.contains("pro w6800") {
        return Some(32);
    }

    // RX 6000 series (RDNA 2)
    if name_lower.contains("6950 xt") || name_lower.contains("6900 xt")
        || name_lower.contains("6800 xt") || name_lower.contains("6800") {
        return Some(16);
    } else if name_lower.contains("6750 xt") || name_lower.contains("6700 xt") {
        return Some(12);
    } else if name_lower.contains("6700") || name_lower.contains("6650 xt") {
        return Some(10);
    } else if name_lower.contains("6600 xt") || name_lower.contains("6600") {
        return Some(8);
    } else if name_lower.contains("6500 xt") || name_lower.contains("6400") {
        return Some(4);
    }

    // RX 5000 series (RDNA 1)
    if name_lower.contains("5700 xt") || name_lower.contains("5700") {
        return Some(8);
    } else if name_lower.contains("5600 xt") || name_lower.contains("5600") {
        return Some(6);
    } else if name_lower.contains("5500 xt") {
        return Some(8); // Could be 4GB or 8GB variant
    }

    // Radeon VII
    if name_lower.contains("radeon vii") || name_lower.contains("radeon 7") {
        return Some(16);
    }

    // Vega series (both 64 and 56 have 8GB)
    if name_lower.contains("vega 64") || name_lower.contains("vega 56") {
        return Some(8);
    }

    None
}

/// Detects AMD GPU on Windows using multiple methods with fallbacks.
///
/// Attempts detection in the following order:
/// 1. DXGI query for DedicatedVideoMemory (most accurate)
/// 2. Registry query for HardwareInformation.qwMemorySize
/// 3. Model name inference for known GPUs
/// 4. WMI AdapterRAM with wraparound handling
fn detect_amd_windows() -> Option<GpuInfo> {
    // First, get the GPU name using WMI
    let gpu_name_output = Command::new("powershell")
        .args([
            "-NoProfile",
            "-Command",
            "$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu) { $gpu.Name }"
        ])
        .output()
        .ok()?;

    if !gpu_name_output.status.success() {
        return None;
    }

    let gpu_name = String::from_utf8_lossy(&gpu_name_output.stdout).trim().to_string();

    if gpu_name.is_empty() {
        return None;
    }

    // Try Method 1: DXGI DedicatedVideoMemory (most accurate for modern GPUs)
    let dxgi_vram = Command::new("powershell")
        .args([
            "-NoProfile",
            "-Command",
            r#"
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public struct DXGI_ADAPTER_DESC {
    [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)]
    public string Description;
    public uint VendorId;
    public uint DeviceId;
    public uint SubSysId;
    public uint Revision;
    public ulong DedicatedVideoMemory;
    public ulong DedicatedSystemMemory;
    public ulong SharedSystemMemory;
    public long AdapterLuid;
}

public class DXGI {
    [DllImport("dxgi.dll")]
    public static extern int CreateDXGIFactory(ref Guid riid, out IntPtr ppFactory);

    [DllImport("dxgi.dll")]
    public static extern int CreateDXGIFactory1(ref Guid riid, out IntPtr ppFactory);
}
"@

try {
    $guid = [Guid]::Parse("7b7166ec-21c7-44ae-b21a-c9ae321ae369")
    $factory = [IntPtr]::Zero
    [DXGI]::CreateDXGIFactory1([ref]$guid, [ref]$factory)
    if ($factory -ne [IntPtr]::Zero) {
        # Get adapter using reflection
        $factoryType = [System.Runtime.InteropServices.Marshal]::GetObjectForIUnknown($factory).GetType()
        $enumAdapters = $factoryType.GetMethod("EnumAdapters")
        $adapter = $null
        $i = 0
        while ($true) {
            try {
                $adapter = $enumAdapters.Invoke([System.Runtime.InteropServices.Marshal]::GetObjectForIUnknown($factory), @($i))
                if ($adapter) {
                    $desc = New-Object DXGI_ADAPTER_DESC
                    $getDesc = $adapter.GetType().GetMethod("GetDesc")
                    $getDesc.Invoke($adapter, @([ref]$desc))
                    if ($desc.Description -like "*AMD*" -or $desc.Description -like "*Radeon*") {
                        $vramGB = [Math]::Round($desc.DedicatedVideoMemory / 1GB, 0)
                        Write-Output $vramGB
                        break
                    }
                }
                $i++
            } catch {
                break
            }
        }
    }
} catch {
    Write-Output "DXGI_FAILED"
}
"#
        ])
        .output()
        .ok()
        .and_then(|out| {
            if out.status.success() {
                let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
                if s != "DXGI_FAILED" && !s.is_empty() {
                    s.parse::<u32>().ok()
                } else {
                    None
                }
            } else {
                None
            }
        });

    // Try Method 2: Registry query (reliable for AMD drivers)
    let registry_vram = if dxgi_vram.is_none() {
        Command::new("powershell")
            .args([
                "-NoProfile",
                "-Command",
                r#"
try {
    $paths = @(
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0000",
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0001",
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0002"
    )
    foreach ($path in $paths) {
        if (Test-Path $path) {
            $desc = (Get-ItemProperty -Path $path -Name "DriverDesc" -ErrorAction SilentlyContinue).DriverDesc
            if ($desc -like "*AMD*" -or $desc -like "*Radeon*") {
                $mem = (Get-ItemProperty -Path $path -Name "HardwareInformation.qwMemorySize" -ErrorAction SilentlyContinue)."HardwareInformation.qwMemorySize"
                if ($mem) {
                    $vramGB = [Math]::Round($mem / 1GB, 0)
                    Write-Output $vramGB
                    exit
                }
            }
        }
    }
    Write-Output "REG_FAILED"
} catch {
    Write-Output "REG_FAILED"
}
"#
            ])
            .output()
            .ok()
            .and_then(|out| {
                if out.status.success() {
                    let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
                    if s != "REG_FAILED" && !s.is_empty() {
                        s.parse::<u32>().ok()
                    } else {
                        None
                    }
                } else {
                    None
                }
            })
    } else {
        None
    };

    // Try Method 3: Infer from GPU model name
    let inferred_vram = if dxgi_vram.is_none() && registry_vram.is_none() {
        infer_amd_vram_from_model(&gpu_name)
    } else {
        None
    };

    // Try Method 4: WMI AdapterRAM as last resort (has 4GB limitation, but handle wraparound)
    let wmi_vram = if dxgi_vram.is_none() && registry_vram.is_none() && inferred_vram.is_none() {
        Command::new("powershell")
            .args([
                "-NoProfile",
                "-Command",
                "$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu -and $gpu.AdapterRAM) { [Math]::Round($gpu.AdapterRAM / 1GB, 0) }"
            ])
            .output()
            .ok()
            .and_then(|out| {
                if out.status.success() {
                    String::from_utf8_lossy(&out.stdout).trim().parse::<u32>().ok()
                } else {
                    None
                }
            })
    } else {
        None
    };

    // Determine final VRAM value with validation
    let mut vram_gb = dxgi_vram
        .or(registry_vram)
        .or(inferred_vram)
        .or(wmi_vram)
        .unwrap_or(8); // Default to 8GB if all methods fail

    // Validation: If WMI returned exactly 4GB for a high-end GPU, it's likely wrong
    // Check if we have an inferred value that's higher
    if vram_gb == 4 {
        if let Some(inferred) = infer_amd_vram_from_model(&gpu_name) {
            if inferred > 4 {
                // High-end GPU detected, WMI is showing 32-bit limitation
                vram_gb = inferred;
            }
        }
    }

    // Get driver version
    let driver = Command::new("powershell")
        .args([
            "-NoProfile",
            "-Command",
            "$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu) { $gpu.DriverVersion }"
        ])
        .output()
        .ok()
        .and_then(|out| {
            if out.status.success() {
                let version = String::from_utf8_lossy(&out.stdout).trim().to_string();
                if !version.is_empty() {
                    Some(version)
                } else {
                    None
                }
            } else {
                None
            }
        });

    Some(GpuInfo {
        name: gpu_name,
        vram_gb,
        driver,
        gpu_type: GpuType::Amd,
    })
}

/// Detects AMD GPUs using rocm-smi on Linux or multiple methods on Windows.
///
/// Queries rocm-smi for GPU information on Linux systems with ROCm installed.
/// On Windows, uses multiple detection methods with fallbacks:
/// 1. DXGI (DirectX Graphics Infrastructure) for accurate VRAM detection
/// 2. Registry queries for video memory information
/// 3. Model name inference for known high-end GPUs
/// 4. WMI with AdapterRAM wraparound handling as last resort
fn detect_amd() -> Option<GpuInfo> {
    // On Windows, use enhanced detection with multiple methods
    if cfg!(target_os = "windows") {
        return detect_amd_windows();
    }

    // On Linux, try rocm-smi
    let output = Command::new("rocm-smi")
        .args(["--showproductname", "--showmeminfo", "vram"])
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout);

    // Parse GPU name
    let name = stdout
        .lines()
        .find(|line| line.contains("Card series:") || line.contains("GPU"))
        .map(|line| {
            line.split(':')
                .nth(1)
                .map(|s| format!("AMD {}", s.trim()))
                .unwrap_or_else(|| "AMD GPU".to_string())
        })
        .unwrap_or_else(|| "AMD GPU".to_string());

    // Parse VRAM (look for total memory)
    let vram_gb = stdout
        .lines()
        .find(|line| line.contains("Total Memory") || line.contains("VRAM Total"))
        .and_then(|line| {
            // Extract numeric value, typically in bytes or MB
            line.split_whitespace()
                .filter_map(|s| s.parse::<u64>().ok())
                .next()
                .map(|bytes| {
                    // Assume bytes if large number, MB if smaller
                    if bytes > 1_000_000_000 {
                        (bytes / 1_073_741_824) as u32 // bytes to GB
                    } else if bytes > 1_000_000 {
                        (bytes / 1024) as u32 // MB to GB
                    } else {
                        bytes as u32 // Already in GB
                    }
                })
        })
        .unwrap_or(8); // Default to 8GB if parsing fails

    // Try to get driver version
    let driver = Command::new("rocm-smi")
        .args(["--showdriverversion"])
        .output()
        .ok()
        .and_then(|out| {
            if out.status.success() {
                let s = String::from_utf8_lossy(&out.stdout);
                s.lines()
                    .find(|line| line.contains("Driver version"))
                    .map(|line| {
                        line.split(':')
                            .nth(1)
                            .map(|s| s.trim().to_string())
                            .unwrap_or_default()
                    })
            } else {
                None
            }
        });

    Some(GpuInfo {
        name,
        vram_gb,
        driver,
        gpu_type: GpuType::Amd,
    })
}

/// Detects Apple Silicon GPU on macOS.
///
/// Uses system_profiler to detect Apple Silicon chips and their unified memory.
fn detect_apple_silicon() -> Option<GpuInfo> {
    // Only available on macOS
    if !cfg!(target_os = "macos") {
        return None;
    }

    let output = Command::new("system_profiler")
        .args(["SPDisplaysDataType", "-json"])
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout);

    // Check if it's Apple Silicon
    if !stdout.contains("Apple") {
        return None;
    }

    // Parse chip name (M1, M2, M3, etc.)
    let name = if stdout.contains("M3 Max") {
        "Apple M3 Max"
    } else if stdout.contains("M3 Pro") {
        "Apple M3 Pro"
    } else if stdout.contains("M3") {
        "Apple M3"
    } else if stdout.contains("M2 Ultra") {
        "Apple M2 Ultra"
    } else if stdout.contains("M2 Max") {
        "Apple M2 Max"
    } else if stdout.contains("M2 Pro") {
        "Apple M2 Pro"
    } else if stdout.contains("M2") {
        "Apple M2"
    } else if stdout.contains("M1 Ultra") {
        "Apple M1 Ultra"
    } else if stdout.contains("M1 Max") {
        "Apple M1 Max"
    } else if stdout.contains("M1 Pro") {
        "Apple M1 Pro"
    } else if stdout.contains("M1") {
        "Apple M1"
    } else {
        "Apple Silicon"
    }
    .to_string();

    // Get unified memory (shared between CPU and GPU)
    // Use sysctl to get total physical memory
    let vram_gb = Command::new("sysctl")
        .args(["-n", "hw.memsize"])
        .output()
        .ok()
        .and_then(|out| {
            if out.status.success() {
                let s = String::from_utf8_lossy(&out.stdout);
                s.trim().parse::<u64>().ok().map(|bytes| {
                    // Apple Silicon can use up to ~75% of unified memory for GPU
                    // Report the full memory as it's shared
                    (bytes / 1_073_741_824) as u32
                })
            } else {
                None
            }
        })
        .unwrap_or(8);

    // Get macOS version as "driver"
    let driver = Command::new("sw_vers")
        .args(["-productVersion"])
        .output()
        .ok()
        .and_then(|out| {
            if out.status.success() {
                Some(format!(
                    "macOS {}",
                    String::from_utf8_lossy(&out.stdout).trim()
                ))
            } else {
                None
            }
        });

    Some(GpuInfo {
        name,
        vram_gb,
        driver,
        gpu_type: GpuType::AppleSilicon,
    })
}

/// Detects Intel Arc GPUs using intel_gpu_top.
///
/// Checks for Intel Arc discrete GPUs on Linux systems.
fn detect_intel_arc() -> Option<GpuInfo> {
    // Try intel_gpu_top to check for Intel GPU
    let output = Command::new("intel_gpu_top")
        .args(["-L"])
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout);

    // Check if it's an Arc GPU (discrete)
    if !stdout.to_lowercase().contains("arc") {
        return None;
    }

    // Parse GPU name
    let name = stdout
        .lines()
        .find(|line| line.to_lowercase().contains("arc"))
        .map(|line| {
            // Extract the Arc model name
            if line.contains("A770") {
                "Intel Arc A770"
            } else if line.contains("A750") {
                "Intel Arc A750"
            } else if line.contains("A580") {
                "Intel Arc A580"
            } else if line.contains("A380") {
                "Intel Arc A380"
            } else if line.contains("A310") {
                "Intel Arc A310"
            } else {
                "Intel Arc"
            }
        })
        .unwrap_or("Intel Arc")
        .to_string();

    // Estimate VRAM based on model
    let vram_gb = match name.as_str() {
        "Intel Arc A770" => 16,
        "Intel Arc A750" => 8,
        "Intel Arc A580" => 8,
        "Intel Arc A380" => 6,
        "Intel Arc A310" => 4,
        _ => 8, // Default estimate
    };

    Some(GpuInfo {
        name,
        vram_gb,
        driver: None,
        gpu_type: GpuType::Intel,
    })
}

/// Recommends an AI model based on available VRAM.
///
/// This function returns the optimal model for code generation tasks
/// based on the available GPU memory.
///
/// # Model Recommendations (2025 benchmarks)
/// - **0-5GB**: `qwen2.5-coder:3b` - Lightweight, suitable for basic coding tasks
/// - **6-9GB**: `qwen2.5-coder:7b` - Excellent all-rounder, near GPT-4o level
/// - **10-17GB**: `qwen2.5-coder:14b` - Best performance per VRAM
/// - **18-26GB**: `qwen2.5-coder:32b` - Competitive with GPT-4o on code repair
/// - **27-47GB**: `qwen3-coder:30b` - MoE architecture, 256K context
/// - **48GB+**: `qwen3-coder:30b` - Full Qwen 3 Coder experience
///
/// # Arguments
/// * `vram_gb` - Available VRAM in gigabytes
///
/// # Returns
/// The recommended model name as a string.
///
/// # Example
/// ```rust
/// use rigrun::detect::recommend_model;
///
/// let model = recommend_model(16);
/// assert_eq!(model, "qwen2.5-coder:14b");
/// ```
/// Recommends the optimal coding model based on available VRAM.
///
/// Model recommendations are based on 2025 benchmarks:
/// - Qwen 2.5 Coder consistently outperforms other models in code generation
/// - Codestral excels at fill-in-the-middle (autocomplete) tasks
/// - Qwen 3 Coder (MoE) offers best performance for high-VRAM systems
///
/// # Arguments
/// * `vram_gb` - Available VRAM in gigabytes
///
/// # Returns
/// The recommended model name optimized for coding tasks.
pub fn recommend_model(vram_gb: u32) -> String {
    match vram_gb {
        // Very limited VRAM: smallest capable model
        0..=5 => "qwen2.5-coder:3b".to_string(),
        // 8GB class: 7B model benchmarks near GPT-4o in some tests
        6..=9 => "qwen2.5-coder:7b".to_string(),
        // 12-16GB class: 14B is the sweet spot, outperforms DeepSeek
        10..=17 => "qwen2.5-coder:14b".to_string(),
        // 18-24GB class: 32B is competitive with GPT-4o on code repair
        18..=26 => "qwen2.5-coder:32b".to_string(),
        // 32GB+ class: Qwen 3 Coder MoE with 256K context
        27..=47 => "qwen3-coder:30b".to_string(),
        // 48GB+ class: Full Qwen 3 Coder experience
        _ => "qwen3-coder:30b".to_string(),
    }
}

/// Returns alternative model recommendations for the given VRAM.
///
/// Provides options optimized for different use cases:
/// - Fill-in-the-middle (autocomplete): Codestral
/// - General coding: Qwen 2.5 Coder
/// - Long context: Qwen 3 Coder
pub fn recommend_models_all(vram_gb: u32) -> Vec<ModelRecommendation> {
    match vram_gb {
        0..=5 => vec![
            ModelRecommendation::new("qwen2.5-coder:3b", "Best for limited VRAM", true),
        ],
        6..=9 => vec![
            ModelRecommendation::new("qwen2.5-coder:7b", "Excellent all-rounder, near GPT-4o level", true),
            ModelRecommendation::new("deepseek-coder-v2:lite", "Good alternative", false),
        ],
        10..=17 => vec![
            ModelRecommendation::new("qwen2.5-coder:14b", "Best performance per VRAM", true),
            ModelRecommendation::new("codestral:22b", "Best for autocomplete (FIM)", false),
            ModelRecommendation::new("deepseek-coder-v2:16b", "Strong debugging partner", false),
        ],
        18..=26 => vec![
            ModelRecommendation::new("qwen2.5-coder:32b", "Competitive with GPT-4o", true),
            ModelRecommendation::new("codestral:22b", "Best for autocomplete (FIM)", false),
            ModelRecommendation::new("qwen2.5-coder:14b", "Faster, leaves headroom", false),
        ],
        _ => vec![
            ModelRecommendation::new("qwen3-coder:30b", "Latest MoE, 256K context", true),
            ModelRecommendation::new("qwen2.5-coder:32b", "Proven performer", false),
        ],
    }
}

/// A model recommendation with metadata.
#[derive(Debug, Clone)]
pub struct ModelRecommendation {
    /// The model name (e.g., "qwen2.5-coder:14b")
    pub name: String,
    /// Why this model is recommended
    pub reason: String,
    /// Whether this is the primary recommendation
    pub is_primary: bool,
}

impl ModelRecommendation {
    /// Creates a new model recommendation.
    pub fn new(name: &str, reason: &str, is_primary: bool) -> Self {
        Self {
            name: name.to_string(),
            reason: reason.to_string(),
            is_primary,
        }
    }
}

/// Returns an alternative model recommendation for the given VRAM.
///
/// Provides a secondary option that may be preferred in certain scenarios.
///
/// # Arguments
/// * `vram_gb` - Available VRAM in gigabytes
///
/// # Returns
/// An alternative model name, or `None` if no alternative is available.
pub fn recommend_model_alt(vram_gb: u32) -> Option<String> {
    match vram_gb {
        16..=23 => Some("qwen2.5-coder:32b".to_string()),
        _ => None,
    }
}

/// Checks if Ollama is available on the system.
///
/// Verifies that the Ollama CLI is installed and accessible.
///
/// # Returns
/// `true` if Ollama is available, `false` otherwise.
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::check_ollama_available;
///
/// if check_ollama_available() {
///     println!("Ollama is ready!");
/// } else {
///     println!("Please install Ollama first.");
/// }
/// ```
pub fn check_ollama_available() -> bool {
    Command::new("ollama")
        .arg("--version")
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

/// Checks if Ollama server is running and responsive.
///
/// Attempts to connect to the Ollama API to verify the server is active.
///
/// # Returns
/// `true` if Ollama server is running, `false` otherwise.
pub fn check_ollama_running() -> bool {
    Command::new("ollama")
        .arg("list")
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

/// Lists all models currently available in Ollama.
///
/// # Returns
/// A vector of model names, or an empty vector if Ollama is not available.
pub fn list_ollama_models() -> Vec<String> {
    Command::new("ollama")
        .arg("list")
        .output()
        .ok()
        .map(|output| {
            if output.status.success() {
                String::from_utf8_lossy(&output.stdout)
                    .lines()
                    .skip(1) // Skip header line
                    .filter_map(|line| {
                        line.split_whitespace().next().map(|s| s.to_string())
                    })
                    .collect()
            } else {
                Vec::new()
            }
        })
        .unwrap_or_default()
}

/// Checks if a specific model is available in Ollama.
///
/// # Arguments
/// * `model_name` - The name of the model to check
///
/// # Returns
/// `true` if the model is available, `false` otherwise.
pub fn is_model_available(model_name: &str) -> bool {
    list_ollama_models()
        .iter()
        .any(|m| m.starts_with(model_name) || m == model_name)
}

/// Represents the processor type being used by a loaded model.
#[derive(Debug, Clone, PartialEq)]
pub enum ProcessorType {
    /// Model is running entirely on GPU
    Gpu(u8), // GPU percentage (typically 100)
    /// Model is running entirely on CPU
    Cpu,
    /// Model is running on a mix of GPU and CPU
    Mixed { gpu_percent: u8, cpu_percent: u8 },
    /// Unable to determine processor type
    Unknown,
}

impl std::fmt::Display for ProcessorType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ProcessorType::Gpu(pct) => write!(f, "{}% GPU", pct),
            ProcessorType::Cpu => write!(f, "100% CPU"),
            ProcessorType::Mixed { gpu_percent, cpu_percent } => {
                write!(f, "{}% GPU/{}% CPU", gpu_percent, cpu_percent)
            }
            ProcessorType::Unknown => write!(f, "Unknown"),
        }
    }
}

/// Information about a model loaded in Ollama.
#[derive(Debug, Clone)]
pub struct LoadedModelInfo {
    /// The model name
    pub name: String,
    /// The model ID
    pub id: String,
    /// Size of the model in memory (as string, e.g., "9.0 GB")
    pub size: String,
    /// Size in bytes (parsed from size string)
    pub size_bytes: u64,
    /// The processor type being used
    pub processor: ProcessorType,
    /// Time until the model is unloaded
    pub until: String,
}

/// Parses the output of `ollama ps` to get information about loaded models.
///
/// # Returns
/// A vector of `LoadedModelInfo` for each model currently loaded in Ollama.
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::get_ollama_loaded_models;
///
/// let models = get_ollama_loaded_models();
/// for model in models {
///     println!("{}: {}", model.name, model.processor);
/// }
/// ```
pub fn get_ollama_loaded_models() -> Vec<LoadedModelInfo> {
    let output = match Command::new("ollama").arg("ps").output() {
        Ok(out) if out.status.success() => out,
        _ => return Vec::new(),
    };

    let stdout = String::from_utf8_lossy(&output.stdout);
    let mut models = Vec::new();

    // Skip header line
    for line in stdout.lines().skip(1) {
        if line.trim().is_empty() {
            continue;
        }

        // Parse the line - columns are: NAME, ID, SIZE, PROCESSOR, UNTIL
        // The output is whitespace-separated but some fields may contain spaces
        let parts: Vec<&str> = line.split_whitespace().collect();

        if parts.len() < 5 {
            continue;
        }

        let name = parts[0].to_string();
        let id = parts[1].to_string();

        // Size is typically "X.X GB" - find GB/MB position
        let mut size_idx = 2;
        let mut size_parts = vec![parts[size_idx]];
        if parts.len() > size_idx + 1
            && (parts[size_idx + 1].to_uppercase() == "GB"
                || parts[size_idx + 1].to_uppercase() == "MB")
        {
            size_parts.push(parts[size_idx + 1]);
            size_idx += 2;
        } else {
            size_idx += 1;
        }
        let size = size_parts.join(" ");

        // Parse size to bytes
        let size_bytes = parse_size_to_bytes(&size);

        // Processor is typically "100% GPU" or "100% CPU" or "50% GPU/50% CPU"
        let processor_idx = size_idx;
        let processor = if parts.len() > processor_idx {
            parse_processor(&parts[processor_idx..])
        } else {
            ProcessorType::Unknown
        };

        // Until is the remaining text
        let until_start = if parts.len() > processor_idx + 2 {
            processor_idx + 2
        } else {
            parts.len()
        };
        let until = if until_start < parts.len() {
            parts[until_start..].join(" ")
        } else {
            String::new()
        };

        models.push(LoadedModelInfo {
            name,
            id,
            size,
            size_bytes,
            processor,
            until,
        });
    }

    models
}

/// Parses a size string like "9.0 GB" or "512 MB" to bytes.
fn parse_size_to_bytes(size: &str) -> u64 {
    let parts: Vec<&str> = size.split_whitespace().collect();
    if parts.len() < 2 {
        // Try parsing as a single number (bytes)
        return parts.first().and_then(|s| s.parse::<u64>().ok()).unwrap_or(0);
    }

    let value: f64 = parts[0].parse().unwrap_or(0.0);
    let unit = parts[1].to_uppercase();

    match unit.as_str() {
        "GB" => (value * 1024.0 * 1024.0 * 1024.0) as u64,
        "MB" => (value * 1024.0 * 1024.0) as u64,
        "KB" => (value * 1024.0) as u64,
        _ => value as u64,
    }
}

/// Parses processor information from ollama ps output.
fn parse_processor(parts: &[&str]) -> ProcessorType {
    if parts.is_empty() {
        return ProcessorType::Unknown;
    }

    let processor_str = parts.join(" ");
    let processor_upper = processor_str.to_uppercase();

    // Check for GPU percentage
    if processor_upper.contains("GPU") {
        // Try to extract percentage
        if let Some(pct_str) = processor_str.split('%').next() {
            if let Ok(pct) = pct_str.trim().parse::<u8>() {
                if processor_upper.contains("CPU") {
                    // Mixed mode - try to extract CPU percentage too
                    if let Some(cpu_part) = processor_str.split('/').nth(1) {
                        if let Some(cpu_pct_str) = cpu_part.split('%').next() {
                            if let Ok(cpu_pct) = cpu_pct_str.trim().parse::<u8>() {
                                return ProcessorType::Mixed {
                                    gpu_percent: pct,
                                    cpu_percent: cpu_pct,
                                };
                            }
                        }
                    }
                    return ProcessorType::Mixed {
                        gpu_percent: pct,
                        cpu_percent: 100 - pct,
                    };
                }
                return ProcessorType::Gpu(pct);
            }
        }
        return ProcessorType::Gpu(100);
    } else if processor_upper.contains("CPU") {
        return ProcessorType::Cpu;
    }

    ProcessorType::Unknown
}

/// Checks if a specific model is currently loaded in Ollama and returns its status.
///
/// # Arguments
/// * `model_name` - The name of the model to check
///
/// # Returns
/// `Some(LoadedModelInfo)` if the model is loaded, `None` otherwise.
pub fn get_model_gpu_status(model_name: &str) -> Option<LoadedModelInfo> {
    let models = get_ollama_loaded_models();
    models.into_iter().find(|m| {
        m.name == model_name
            || m.name.starts_with(&format!("{}:", model_name))
            || model_name.starts_with(&format!("{}:", m.name.split(':').next().unwrap_or("")))
    })
}

/// Checks if a model is running on GPU.
///
/// # Arguments
/// * `model_name` - The name of the model to check
///
/// # Returns
/// - `Some(true)` if the model is loaded and running on GPU
/// - `Some(false)` if the model is loaded but running on CPU
/// - `None` if the model is not currently loaded
pub fn is_model_using_gpu(model_name: &str) -> Option<bool> {
    get_model_gpu_status(model_name).map(|info| match info.processor {
        ProcessorType::Gpu(_) => true,
        ProcessorType::Mixed { gpu_percent, .. } => gpu_percent > 0,
        ProcessorType::Cpu => false,
        ProcessorType::Unknown => false,
    })
}

/// Estimates the VRAM required for a model based on its parameter count.
///
/// This is a rough estimate using the following heuristics:
/// - Q4_K_M quantization uses approximately 0.5-0.6 bytes per parameter
/// - Q8_0 quantization uses approximately 1 byte per parameter
/// - FP16 uses approximately 2 bytes per parameter
///
/// We estimate based on common model naming conventions (e.g., "7b" = 7 billion parameters).
///
/// # Arguments
/// * `model_name` - The model name (e.g., "qwen2.5-coder:14b")
///
/// # Returns
/// Estimated VRAM in GB, or None if unable to estimate.
pub fn estimate_model_vram(model_name: &str) -> Option<u32> {
    let name_lower = model_name.to_lowercase();

    // Extract parameter count from model name
    let param_billions: f64 = if name_lower.contains("1.5b") || name_lower.contains(":1.5b") {
        1.5
    } else if name_lower.contains("3b") || name_lower.contains(":3b") {
        3.0
    } else if name_lower.contains("7b") || name_lower.contains(":7b") {
        7.0
    } else if name_lower.contains("8b") || name_lower.contains(":8b") {
        8.0
    } else if name_lower.contains("13b") || name_lower.contains(":13b") {
        13.0
    } else if name_lower.contains("14b") || name_lower.contains(":14b") {
        14.0
    } else if name_lower.contains("16b") || name_lower.contains(":16b") {
        16.0
    } else if name_lower.contains("22b") || name_lower.contains(":22b") {
        22.0
    } else if name_lower.contains("30b") || name_lower.contains(":30b") {
        30.0
    } else if name_lower.contains("32b") || name_lower.contains(":32b") {
        32.0
    } else if name_lower.contains("34b") || name_lower.contains(":34b") {
        34.0
    } else if name_lower.contains("70b") || name_lower.contains(":70b") {
        70.0
    } else if name_lower.contains("72b") || name_lower.contains(":72b") {
        72.0
    } else {
        return None;
    };

    // Estimate VRAM based on Q4_K_M quantization (most common in Ollama)
    // Q4_K_M uses roughly 4.5 bits per parameter = 0.5625 bytes per parameter
    // Add ~1-2GB overhead for KV cache and CUDA context
    let bytes_per_param = 0.56;
    let overhead_gb = 1.5;

    let vram_gb = (param_billions * bytes_per_param) + overhead_gb;

    Some(vram_gb.ceil() as u32)
}

/// Checks if a model will likely fit in the available VRAM.
///
/// # Arguments
/// * `model_name` - The model name (e.g., "qwen2.5-coder:14b")
/// * `available_vram_gb` - Available VRAM in GB
///
/// # Returns
/// - `Ok(true)` if the model should fit
/// - `Ok(false)` if the model is too large
/// - `Err` if unable to estimate
pub fn will_model_fit_in_vram(model_name: &str, available_vram_gb: u32) -> std::result::Result<bool, String> {
    match estimate_model_vram(model_name) {
        Some(required) => Ok(required <= available_vram_gb),
        None => Err(format!("Unable to estimate VRAM for model: {}", model_name)),
    }
}

/// GPU utilization status for display purposes.
#[derive(Debug, Clone)]
pub struct GpuUtilizationStatus {
    /// Whether the model is using GPU
    pub using_gpu: bool,
    /// The processor type being used
    pub processor: ProcessorType,
    /// Available VRAM in GB
    pub available_vram_gb: u32,
    /// Estimated VRAM required by the model in GB
    pub required_vram_gb: Option<u32>,
    /// Warning message if any
    pub warning: Option<String>,
    /// Suggested smaller model if applicable
    pub suggested_model: Option<String>,
}

/// Real-time GPU memory usage information.
#[derive(Debug, Clone)]
pub struct GpuMemoryUsage {
    /// Total VRAM in MB
    pub total_mb: u64,
    /// Used VRAM in MB
    pub used_mb: u64,
    /// Free VRAM in MB
    pub free_mb: u64,
    /// GPU utilization percentage (0-100)
    pub gpu_utilization: Option<u8>,
}

impl GpuMemoryUsage {
    /// Returns total VRAM in GB (rounded).
    pub fn total_gb(&self) -> u32 {
        (self.total_mb as f64 / 1024.0).round() as u32
    }

    /// Returns used VRAM in GB (rounded).
    pub fn used_gb(&self) -> u32 {
        (self.used_mb as f64 / 1024.0).round() as u32
    }

    /// Returns free VRAM in GB (rounded).
    pub fn free_gb(&self) -> u32 {
        (self.free_mb as f64 / 1024.0).round() as u32
    }

    /// Returns the VRAM usage percentage.
    pub fn usage_percent(&self) -> f64 {
        if self.total_mb == 0 {
            0.0
        } else {
            (self.used_mb as f64 / self.total_mb as f64) * 100.0
        }
    }
}

/// Gets real-time GPU memory usage from nvidia-smi.
///
/// # Returns
/// `Some(GpuMemoryUsage)` if NVIDIA GPU is available and nvidia-smi succeeds,
/// `None` otherwise.
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::get_nvidia_gpu_memory_usage;
///
/// if let Some(usage) = get_nvidia_gpu_memory_usage() {
///     println!("VRAM: {}GB used / {}GB total ({:.1}%)",
///         usage.used_gb(), usage.total_gb(), usage.usage_percent());
/// }
/// ```
pub fn get_nvidia_gpu_memory_usage() -> Option<GpuMemoryUsage> {
    let nvidia_smi_paths = if cfg!(target_os = "windows") {
        vec![
            "nvidia-smi",
            "C:\\Windows\\System32\\nvidia-smi.exe",
            "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
        ]
    } else {
        vec!["nvidia-smi"]
    };

    let mut output = None;
    for path in nvidia_smi_paths {
        if let Ok(out) = Command::new(path)
            .args([
                "--query-gpu=memory.total,memory.used,memory.free,utilization.gpu",
                "--format=csv,noheader,nounits",
            ])
            .output()
        {
            if out.status.success() {
                output = Some(out);
                break;
            }
        }
    }

    let output = output?;
    let stdout = String::from_utf8_lossy(&output.stdout);
    let line = stdout.lines().next()?.trim();
    let parts: Vec<&str> = line.split(", ").collect();

    if parts.len() < 3 {
        return None;
    }

    let total_mb: u64 = parts[0].trim().parse().ok()?;
    let used_mb: u64 = parts[1].trim().parse().ok()?;
    let free_mb: u64 = parts[2].trim().parse().ok()?;
    let gpu_utilization: Option<u8> = parts.get(3).and_then(|s| s.trim().parse().ok());

    Some(GpuMemoryUsage {
        total_mb,
        used_mb,
        free_mb,
        gpu_utilization,
    })
}

/// Gets GPU memory usage for AMD GPUs on Windows.
///
/// Note: AMD doesn't have a universal memory query tool like nvidia-smi,
/// so this function attempts to use Windows performance counters or registry.
pub fn get_amd_gpu_memory_usage() -> Option<GpuMemoryUsage> {
    // AMD doesn't have a reliable cross-platform memory query tool
    // On Windows, we can try to use PowerShell to query video memory
    if !cfg!(target_os = "windows") {
        return None;
    }

    // Try to get memory info from WMI
    let output = Command::new("powershell")
        .args([
            "-NoProfile",
            "-Command",
            r#"
$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1
if ($gpu) {
    # AdapterRAM gives total, but we can't easily get used memory
    $totalMB = [Math]::Round($gpu.AdapterRAM / 1MB, 0)
    Write-Output "$totalMB,0,0"
}
"#,
        ])
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    let parts: Vec<&str> = stdout.trim().split(',').collect();

    if parts.len() < 3 {
        return None;
    }

    let total_mb: u64 = parts[0].trim().parse().ok()?;
    // AMD doesn't provide used/free easily, so we return 0 for those
    let used_mb: u64 = parts[1].trim().parse().unwrap_or(0);
    let free_mb: u64 = if used_mb > 0 {
        total_mb.saturating_sub(used_mb)
    } else {
        total_mb // Assume all is free if we can't determine
    };

    Some(GpuMemoryUsage {
        total_mb,
        used_mb,
        free_mb,
        gpu_utilization: None,
    })
}

/// Gets GPU memory usage based on detected GPU type.
///
/// This function automatically detects the GPU type and queries the appropriate
/// system tool to get memory usage information.
///
/// # Arguments
/// * `gpu_info` - Optional GPU info; if None, will detect automatically
///
/// # Returns
/// `Some(GpuMemoryUsage)` if memory information is available, `None` otherwise.
pub fn get_gpu_memory_usage(gpu_info: Option<&GpuInfo>) -> Option<GpuMemoryUsage> {
    let gpu = if let Some(info) = gpu_info {
        info.clone()
    } else {
        detect_gpu().ok()?
    };

    match gpu.gpu_type {
        GpuType::Nvidia => get_nvidia_gpu_memory_usage(),
        GpuType::Amd => get_amd_gpu_memory_usage(),
        GpuType::AppleSilicon => {
            // Apple Silicon uses unified memory - report system memory
            // For now, return None as memory tracking is complex on Apple Silicon
            None
        }
        GpuType::Intel => {
            // Intel Arc doesn't have a standard memory query tool
            None
        }
        GpuType::Cpu => None,
    }
}

/// Performs a comprehensive GPU utilization check for a model.
///
/// This function checks:
/// 1. Whether the model is loaded in Ollama
/// 2. If loaded, whether it's using GPU or CPU
/// 3. If not using GPU, estimates why and suggests alternatives
///
/// # Arguments
/// * `model_name` - The model name to check
/// * `gpu_info` - Information about the detected GPU
///
/// # Returns
/// A `GpuUtilizationStatus` with details about GPU usage.
pub fn check_gpu_utilization(model_name: &str, gpu_info: &GpuInfo) -> GpuUtilizationStatus {
    let available_vram = gpu_info.vram_gb;
    let required_vram = estimate_model_vram(model_name);

    // Check if model is loaded and what processor it's using
    if let Some(loaded_info) = get_model_gpu_status(model_name) {
        let using_gpu = match &loaded_info.processor {
            ProcessorType::Gpu(_) => true,
            ProcessorType::Mixed { gpu_percent, .. } => *gpu_percent > 0,
            ProcessorType::Cpu => false,
            ProcessorType::Unknown => false,
        };

        let warning = if !using_gpu {
            let mut msg = "Model is running on CPU - inference will be slow.".to_string();
            if let Some(req) = required_vram {
                if req > available_vram {
                    msg.push_str(&format!(
                        " Model requires ~{}GB VRAM, but only {}GB available.",
                        req, available_vram
                    ));
                }
            }
            Some(msg)
        } else {
            None
        };

        let suggested_model = if !using_gpu {
            Some(recommend_model(available_vram))
        } else {
            None
        };

        GpuUtilizationStatus {
            using_gpu,
            processor: loaded_info.processor,
            available_vram_gb: available_vram,
            required_vram_gb: required_vram,
            warning,
            suggested_model,
        }
    } else {
        // Model not loaded yet - estimate based on VRAM requirements
        let will_fit = required_vram.map(|r| r <= available_vram).unwrap_or(true);

        let warning = if !will_fit {
            Some(format!(
                "Model may not fit in VRAM. Requires ~{}GB, available: {}GB. May fall back to CPU.",
                required_vram.unwrap_or(0),
                available_vram
            ))
        } else {
            None
        };

        let suggested_model = if !will_fit {
            Some(recommend_model(available_vram))
        } else {
            None
        };

        GpuUtilizationStatus {
            using_gpu: will_fit && gpu_info.gpu_type != GpuType::Cpu,
            processor: if will_fit && gpu_info.gpu_type != GpuType::Cpu {
                ProcessorType::Unknown // Will likely use GPU when loaded
            } else {
                ProcessorType::Cpu
            },
            available_vram_gb: available_vram,
            required_vram_gb: required_vram,
            warning,
            suggested_model,
        }
    }
}

/// Result of a post-load GPU check.
#[derive(Debug, Clone)]
pub struct PostLoadGpuCheck {
    /// Whether the model was found loaded in Ollama
    pub model_loaded: bool,
    /// Whether the model is using GPU
    pub using_gpu: bool,
    /// The processor type if loaded
    pub processor: Option<ProcessorType>,
    /// Size of the model in memory (from ollama ps)
    pub model_size: Option<String>,
    /// Real-time VRAM usage if available
    pub vram_usage: Option<GpuMemoryUsage>,
    /// Warning message if model is on CPU
    pub warning: Option<String>,
    /// Suggested smaller model
    pub suggested_model: Option<String>,
}

/// Checks GPU utilization after a model has been loaded.
///
/// This function is designed to be called after the model has been loaded
/// into Ollama to verify whether it's actually using GPU or has fallen back to CPU.
///
/// # Arguments
/// * `model_name` - The name of the model to check
/// * `gpu_info` - Information about the detected GPU
///
/// # Returns
/// A `PostLoadGpuCheck` with detailed information about GPU usage.
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::{detect_gpu, check_loaded_model_gpu_usage};
///
/// let gpu = detect_gpu().unwrap_or_default();
/// let check = check_loaded_model_gpu_usage("qwen2.5-coder:14b", &gpu);
///
/// if let Some(ref warning) = check.warning {
///     println!("[!] Warning: {}", warning);
/// }
/// ```
pub fn check_loaded_model_gpu_usage(model_name: &str, gpu_info: &GpuInfo) -> PostLoadGpuCheck {
    // Get current VRAM usage
    let vram_usage = get_gpu_memory_usage(Some(gpu_info));

    // Check if model is loaded in Ollama
    if let Some(loaded_info) = get_model_gpu_status(model_name) {
        let using_gpu = match &loaded_info.processor {
            ProcessorType::Gpu(_) => true,
            ProcessorType::Mixed { gpu_percent, .. } => *gpu_percent > 0,
            ProcessorType::Cpu => false,
            ProcessorType::Unknown => false,
        };

        let warning = if !using_gpu {
            let mut msg = "Model is running on CPU - inference will be slow.".to_string();

            // Add VRAM info if available
            if let Some(ref usage) = vram_usage {
                msg.push_str(&format!(
                    " VRAM: {}MB used / {}MB total ({:.1}%).",
                    usage.used_mb, usage.total_mb, usage.usage_percent()
                ));
            }

            // Add model size vs available VRAM info
            if let Some(required) = estimate_model_vram(model_name) {
                if required > gpu_info.vram_gb {
                    msg.push_str(&format!(
                        " Model requires ~{}GB, only {}GB available.",
                        required, gpu_info.vram_gb
                    ));
                }
            }

            Some(msg)
        } else {
            None
        };

        let suggested_model = if !using_gpu {
            Some(recommend_model(gpu_info.vram_gb))
        } else {
            None
        };

        PostLoadGpuCheck {
            model_loaded: true,
            using_gpu,
            processor: Some(loaded_info.processor),
            model_size: Some(loaded_info.size),
            vram_usage,
            warning,
            suggested_model,
        }
    } else {
        // Model not loaded
        PostLoadGpuCheck {
            model_loaded: false,
            using_gpu: false,
            processor: None,
            model_size: None,
            vram_usage,
            warning: None,
            suggested_model: None,
        }
    }
}

/// Formats a warning message for CPU fallback.
///
/// Creates a formatted multi-line warning suitable for terminal output.
///
/// # Arguments
/// * `model_name` - The model that's running on CPU
/// * `available_vram_gb` - Available VRAM in GB
/// * `required_vram_gb` - Estimated required VRAM in GB (optional)
/// * `suggested_model` - Suggested alternative model (optional)
///
/// # Returns
/// A vector of warning message lines.
pub fn format_cpu_fallback_warning(
    model_name: &str,
    available_vram_gb: u32,
    required_vram_gb: Option<u32>,
    suggested_model: Option<&str>,
) -> Vec<String> {
    let mut lines = Vec::new();

    lines.push(format!(
        "[!] Warning: Model '{}' may be running on CPU - inference will be slow",
        model_name
    ));

    if let Some(required) = required_vram_gb {
        lines.push(format!(
            "    Available VRAM: {}GB, Model requires: ~{}GB",
            available_vram_gb, required
        ));
    }

    if let Some(suggested) = suggested_model {
        lines.push(format!(
            "    Consider using a smaller model: rigrun config --model {}",
            suggested
        ));
    }

    lines
}

/// Comprehensive GPU status report for the `rigrun status` command.
#[derive(Debug, Clone)]
pub struct GpuStatusReport {
    /// Information about the detected GPU
    pub gpu_info: GpuInfo,
    /// Real-time VRAM usage if available
    pub vram_usage: Option<GpuMemoryUsage>,
    /// List of models currently loaded in Ollama
    pub loaded_models: Vec<LoadedModelInfo>,
    /// Any warnings about GPU usage
    pub warnings: Vec<String>,
}

/// Represents why a model might be running on CPU when VRAM is sufficient.
#[derive(Debug, Clone, PartialEq)]
pub enum CpuFallbackCause {
    /// AMD GPU needs ollama-for-amd fork (RDNA4, etc.)
    AmdNeedsCustomOllama { gfx_version: String },
    /// ROCm/HIP not installed for AMD
    AmdMissingRocm,
    /// HSA_OVERRIDE_GFX_VERSION might help
    AmdNeedsHsaOverride { suggested_version: String },
    /// NVIDIA CUDA drivers missing or outdated
    NvidiaMissingCuda,
    /// Ollama not compiled with GPU support
    OllamaNoGpuSupport,
    /// Unknown cause
    Unknown,
}

impl std::fmt::Display for CpuFallbackCause {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CpuFallbackCause::AmdNeedsCustomOllama { gfx_version } => {
                write!(f, "AMD GPU ({}) requires ollama-for-amd fork", gfx_version)
            }
            CpuFallbackCause::AmdMissingRocm => {
                write!(f, "ROCm/HIP runtime not installed for AMD GPU")
            }
            CpuFallbackCause::AmdNeedsHsaOverride { suggested_version } => {
                write!(f, "AMD GPU may need HSA_OVERRIDE_GFX_VERSION={}", suggested_version)
            }
            CpuFallbackCause::NvidiaMissingCuda => {
                write!(f, "NVIDIA CUDA drivers missing or outdated")
            }
            CpuFallbackCause::OllamaNoGpuSupport => {
                write!(f, "Ollama may not be compiled with GPU support")
            }
            CpuFallbackCause::Unknown => {
                write!(f, "Unknown cause")
            }
        }
    }
}

/// Diagnosis result for CPU fallback when VRAM is sufficient.
#[derive(Debug, Clone)]
pub struct CpuFallbackDiagnosis {
    /// Whether the GPU has sufficient VRAM for the model
    pub has_sufficient_vram: bool,
    /// Whether the model is currently running on CPU
    pub model_on_cpu: bool,
    /// The likely cause of the CPU fallback
    pub likely_cause: CpuFallbackCause,
    /// Steps to fix the issue
    pub fix_steps: Vec<String>,
}

/// Guidance for setting up GPU acceleration for Ollama.
#[derive(Debug, Clone)]
pub struct GpuSetupGuidance {
    /// Description of the issue detected
    pub issue: String,
    /// Recommended solution
    pub solution: String,
    /// Commands to run to fix the issue
    pub commands: Vec<String>,
    /// Helpful links for more information
    pub links: Vec<String>,
}

/// AMD GPU architecture generation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AmdArchitecture {
    /// RDNA 4 (RX 9000 series, gfx1200/gfx1201)
    Rdna4,
    /// RDNA 3 (RX 7000 series, gfx1100/gfx1101/gfx1102)
    Rdna3,
    /// RDNA 2 (RX 6000 series, gfx1030/gfx1031/gfx1032)
    Rdna2,
    /// RDNA 1 (RX 5000 series, gfx1010/gfx1011/gfx1012)
    Rdna1,
    /// Vega (gfx900/gfx906)
    Vega,
    /// Unknown or unsupported architecture
    Unknown,
}

impl std::fmt::Display for AmdArchitecture {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            AmdArchitecture::Rdna4 => write!(f, "RDNA 4"),
            AmdArchitecture::Rdna3 => write!(f, "RDNA 3"),
            AmdArchitecture::Rdna2 => write!(f, "RDNA 2"),
            AmdArchitecture::Rdna1 => write!(f, "RDNA 1"),
            AmdArchitecture::Vega => write!(f, "Vega"),
            AmdArchitecture::Unknown => write!(f, "Unknown"),
        }
    }
}

/// Detects the AMD GPU architecture from the GPU name.
pub fn detect_amd_architecture(gpu_name: &str) -> AmdArchitecture {
    let name_lower = gpu_name.to_lowercase();

    // RDNA 4 (RX 9000 series)
    if name_lower.contains("9070") || name_lower.contains("9080") || name_lower.contains("9090")
        || name_lower.contains("gfx1200") || name_lower.contains("gfx1201")
    {
        return AmdArchitecture::Rdna4;
    }

    // RDNA 3 (RX 7000 series)
    if name_lower.contains("7900") || name_lower.contains("7800") || name_lower.contains("7700")
        || name_lower.contains("7600") || name_lower.contains("7500")
        || name_lower.contains("gfx1100") || name_lower.contains("gfx1101") || name_lower.contains("gfx1102")
    {
        return AmdArchitecture::Rdna3;
    }

    // RDNA 2 (RX 6000 series)
    if name_lower.contains("6900") || name_lower.contains("6800") || name_lower.contains("6700")
        || name_lower.contains("6600") || name_lower.contains("6500") || name_lower.contains("6400")
        || name_lower.contains("gfx1030") || name_lower.contains("gfx1031") || name_lower.contains("gfx1032")
    {
        return AmdArchitecture::Rdna2;
    }

    // RDNA 1 (RX 5000 series)
    if name_lower.contains("5700") || name_lower.contains("5600") || name_lower.contains("5500")
        || name_lower.contains("gfx1010") || name_lower.contains("gfx1011") || name_lower.contains("gfx1012")
    {
        return AmdArchitecture::Rdna1;
    }

    // Vega
    if name_lower.contains("vega") || name_lower.contains("radeon vii")
        || name_lower.contains("gfx900") || name_lower.contains("gfx906")
    {
        return AmdArchitecture::Vega;
    }

    AmdArchitecture::Unknown
}

/// Checks if ROCm/HIP is installed on the system.
pub fn check_rocm_installed() -> bool {
    // Check for rocm-smi on Linux
    if cfg!(target_os = "linux") {
        if Command::new("rocm-smi").arg("--version").output().is_ok() {
            return true;
        }
        // Also check for HIP runtime
        if std::path::Path::new("/opt/rocm").exists() {
            return true;
        }
    }

    // On Windows, check for HIP SDK
    if cfg!(target_os = "windows") {
        if let Ok(hip_path) = std::env::var("HIP_PATH") {
            if std::path::Path::new(&hip_path).exists() {
                return true;
            }
        }
        // Check common installation paths
        let hip_paths = [
            "C:\\Program Files\\AMD\\ROCm",
            "C:\\Program Files\\AMD\\HIP SDK",
        ];
        for path in hip_paths {
            if std::path::Path::new(path).exists() {
                return true;
            }
        }
    }

    false
}

/// Checks if nvidia-smi is available and working.
pub fn check_nvidia_smi_available() -> bool {
    let nvidia_smi_paths = if cfg!(target_os = "windows") {
        vec![
            "nvidia-smi",
            "C:\\Windows\\System32\\nvidia-smi.exe",
            "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
        ]
    } else {
        vec!["nvidia-smi"]
    };

    for path in nvidia_smi_paths {
        if let Ok(output) = Command::new(path).arg("--version").output() {
            if output.status.success() {
                return true;
            }
        }
    }
    false
}

/// Gets the NVIDIA driver version if available.
pub fn get_nvidia_driver_version() -> Option<String> {
    let nvidia_smi_paths = if cfg!(target_os = "windows") {
        vec![
            "nvidia-smi",
            "C:\\Windows\\System32\\nvidia-smi.exe",
            "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
        ]
    } else {
        vec!["nvidia-smi"]
    };

    for path in nvidia_smi_paths {
        if let Ok(output) = Command::new(path)
            .args(["--query-gpu=driver_version", "--format=csv,noheader,nounits"])
            .output()
        {
            if output.status.success() {
                let version = String::from_utf8_lossy(&output.stdout).trim().to_string();
                if !version.is_empty() {
                    return Some(version);
                }
            }
        }
    }
    None
}

/// Checks if the NVIDIA driver is recent enough for optimal Ollama performance.
/// Ollama generally requires driver 470+ for CUDA 11.4+
pub fn is_nvidia_driver_recent(version: &str) -> bool {
    // Parse the major version number
    if let Some(major_str) = version.split('.').next() {
        if let Ok(major) = major_str.parse::<u32>() {
            // Driver 470+ is recommended for CUDA 11.4+ which Ollama uses
            return major >= 470;
        }
    }
    false
}

/// Gets the recommended HSA_OVERRIDE_GFX_VERSION for an AMD GPU.
/// This can help older or unsupported AMD GPUs work with ROCm/Ollama.
pub fn get_hsa_override_version(gpu_name: &str) -> Option<String> {
    let arch = detect_amd_architecture(gpu_name);

    match arch {
        // RDNA 4 - needs gfx1100 override until ROCm adds native support
        AmdArchitecture::Rdna4 => Some("11.0.0".to_string()),
        // RDNA 3 - usually works natively, but some cards may need override
        AmdArchitecture::Rdna3 => {
            let name_lower = gpu_name.to_lowercase();
            // RX 7600 and some variants may need override
            if name_lower.contains("7600") || name_lower.contains("7700") {
                Some("11.0.0".to_string())
            } else {
                None
            }
        }
        // RDNA 2 - usually works with gfx1030 override
        AmdArchitecture::Rdna2 => {
            let name_lower = gpu_name.to_lowercase();
            if name_lower.contains("6500") || name_lower.contains("6400") {
                // Lower-end RDNA 2 may need override
                Some("10.3.0".to_string())
            } else {
                None
            }
        }
        // RDNA 1 - may need gfx1010 override
        AmdArchitecture::Rdna1 => Some("10.1.0".to_string()),
        // Vega - usually needs gfx900 override
        AmdArchitecture::Vega => Some("9.0.0".to_string()),
        AmdArchitecture::Unknown => None,
    }
}

/// Provides GPU setup guidance based on detected GPU and current status.
///
/// This function analyzes the GPU configuration and provides actionable
/// guidance for setting up GPU acceleration with Ollama.
///
/// # Arguments
/// * `gpu_info` - Information about the detected GPU
///
/// # Returns
/// `Some(GpuSetupGuidance)` if there's an issue that needs addressing,
/// `None` if GPU is properly configured.
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::{detect_gpu, get_gpu_setup_guidance};
///
/// let gpu = detect_gpu().unwrap_or_default();
/// if let Some(guidance) = get_gpu_setup_guidance(&gpu) {
///     println!("Issue: {}", guidance.issue);
///     println!("Solution: {}", guidance.solution);
///     for cmd in &guidance.commands {
///         println!("  Run: {}", cmd);
///     }
/// }
/// ```
pub fn get_gpu_setup_guidance(gpu_info: &GpuInfo) -> Option<GpuSetupGuidance> {
    match gpu_info.gpu_type {
        GpuType::Nvidia => get_nvidia_setup_guidance(gpu_info),
        GpuType::Amd => get_amd_setup_guidance(gpu_info),
        GpuType::AppleSilicon => None, // Apple Silicon generally works out of the box
        GpuType::Intel => get_intel_setup_guidance(gpu_info),
        GpuType::Cpu => Some(GpuSetupGuidance {
            issue: "No GPU detected - Ollama will run on CPU only".to_string(),
            solution: "Install a supported GPU or ensure GPU drivers are properly installed".to_string(),
            commands: vec![],
            links: vec![
                "https://ollama.com/download".to_string(),
                "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
            ],
        }),
    }
}

/// Provides NVIDIA-specific setup guidance.
fn get_nvidia_setup_guidance(gpu_info: &GpuInfo) -> Option<GpuSetupGuidance> {
    // Check if nvidia-smi is available
    if !check_nvidia_smi_available() {
        return Some(GpuSetupGuidance {
            issue: "NVIDIA GPU detected but nvidia-smi is not available".to_string(),
            solution: "Install or update NVIDIA drivers to enable GPU acceleration".to_string(),
            commands: if cfg!(target_os = "windows") {
                vec![
                    "# Download latest drivers from: https://www.nvidia.com/drivers".to_string(),
                    "# Or use GeForce Experience for automatic updates".to_string(),
                ]
            } else {
                vec![
                    "sudo apt update && sudo apt install nvidia-driver-545".to_string(),
                    "# Or download from: https://www.nvidia.com/drivers".to_string(),
                ]
            },
            links: vec![
                "https://www.nvidia.com/drivers".to_string(),
                "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
            ],
        });
    }

    // Check driver version
    if let Some(ref driver) = gpu_info.driver {
        if !is_nvidia_driver_recent(driver) {
            return Some(GpuSetupGuidance {
                issue: format!("NVIDIA driver {} may be outdated for optimal Ollama performance", driver),
                solution: "Update to driver version 470 or newer for best CUDA compatibility".to_string(),
                commands: if cfg!(target_os = "windows") {
                    vec![
                        "# Download latest drivers from: https://www.nvidia.com/drivers".to_string(),
                    ]
                } else {
                    vec![
                        "sudo apt update && sudo apt install nvidia-driver-545".to_string(),
                    ]
                },
                links: vec![
                    "https://www.nvidia.com/drivers".to_string(),
                ],
            });
        }
    }

    // Check if model is running on CPU when GPU is available
    let loaded_models = get_ollama_loaded_models();
    for model in loaded_models {
        if matches!(model.processor, ProcessorType::Cpu) {
            return Some(GpuSetupGuidance {
                issue: format!("Model '{}' is running on CPU despite NVIDIA GPU being available", model.name),
                solution: "The model may be too large for VRAM. Try a smaller model or check CUDA installation.".to_string(),
                commands: vec![
                    format!("# Check VRAM usage: nvidia-smi"),
                    format!("# Try a smaller model: ollama run {}", recommend_model(gpu_info.vram_gb)),
                ],
                links: vec![
                    "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
                ],
            });
        }
    }

    None
}

/// Provides AMD-specific setup guidance.
fn get_amd_setup_guidance(gpu_info: &GpuInfo) -> Option<GpuSetupGuidance> {
    let arch = detect_amd_architecture(&gpu_info.name);

    // RDNA 4 (RX 9070 series) - needs special handling
    if arch == AmdArchitecture::Rdna4 {
        return Some(GpuSetupGuidance {
            issue: format!("{} (RDNA 4) detected - requires ollama-for-amd fork for GPU support", gpu_info.name),
            solution: "RDNA 4 GPUs are not yet supported by official Ollama. Use the community ollama-for-amd fork.".to_string(),
            commands: if cfg!(target_os = "windows") {
                vec![
                    "# Install ollama-for-amd from: https://github.com/likelovewant/ollama-for-amd".to_string(),
                    "# Download the Windows release and extract to a folder".to_string(),
                    "# Run: set HSA_OVERRIDE_GFX_VERSION=11.0.0".to_string(),
                    "# Then run: ollama serve".to_string(),
                ]
            } else {
                vec![
                    "# Clone and build ollama-for-amd:".to_string(),
                    "git clone https://github.com/likelovewant/ollama-for-amd".to_string(),
                    "cd ollama-for-amd && go generate ./... && go build .".to_string(),
                    "export HSA_OVERRIDE_GFX_VERSION=11.0.0".to_string(),
                    "./ollama serve".to_string(),
                ]
            },
            links: vec![
                "https://github.com/likelovewant/ollama-for-amd".to_string(),
                "https://github.com/ollama/ollama/issues/5678".to_string(),
            ],
        });
    }

    // Check if ROCm is installed (Linux) or HIP SDK (Windows)
    if !check_rocm_installed() {
        let (commands, links) = if cfg!(target_os = "linux") {
            (
                vec![
                    "# Install ROCm for AMD GPU support:".to_string(),
                    "wget https://repo.radeon.com/amdgpu-install/latest/ubuntu/jammy/amdgpu-install_6.0.60000-1_all.deb".to_string(),
                    "sudo apt install ./amdgpu-install_6.0.60000-1_all.deb".to_string(),
                    "sudo amdgpu-install --usecase=rocm".to_string(),
                    "sudo usermod -aG render,video $USER".to_string(),
                    "# Log out and back in, then restart Ollama".to_string(),
                ],
                vec![
                    "https://rocm.docs.amd.com/projects/install-on-linux/en/latest/".to_string(),
                    "https://ollama.com/blog/amd-preview".to_string(),
                ],
            )
        } else if cfg!(target_os = "windows") {
            (
                vec![
                    "# AMD GPU support on Windows requires the HIP SDK".to_string(),
                    "# Download from: https://www.amd.com/en/developer/resources/rocm-hub/hip-sdk.html".to_string(),
                    "# After installation, Ollama should detect your AMD GPU".to_string(),
                ],
                vec![
                    "https://www.amd.com/en/developer/resources/rocm-hub/hip-sdk.html".to_string(),
                    "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
                ],
            )
        } else {
            (vec![], vec![])
        };

        if !commands.is_empty() {
            return Some(GpuSetupGuidance {
                issue: format!("{} detected but ROCm/HIP is not installed", gpu_info.name),
                solution: "Install ROCm (Linux) or HIP SDK (Windows) for AMD GPU acceleration".to_string(),
                commands,
                links,
            });
        }
    }

    // Check if HSA_OVERRIDE_GFX_VERSION might help
    if let Some(hsa_version) = get_hsa_override_version(&gpu_info.name) {
        // Check if model is running on CPU
        let loaded_models = get_ollama_loaded_models();
        for model in loaded_models {
            if matches!(model.processor, ProcessorType::Cpu) {
                return Some(GpuSetupGuidance {
                    issue: format!("Model '{}' is running on CPU despite {} being available", model.name, gpu_info.name),
                    solution: format!("Try setting HSA_OVERRIDE_GFX_VERSION={} to enable GPU acceleration", hsa_version),
                    commands: if cfg!(target_os = "windows") {
                        vec![
                            format!("set HSA_OVERRIDE_GFX_VERSION={}", hsa_version),
                            "ollama serve".to_string(),
                        ]
                    } else {
                        vec![
                            format!("export HSA_OVERRIDE_GFX_VERSION={}", hsa_version),
                            "ollama serve".to_string(),
                        ]
                    },
                    links: vec![
                        "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
                        "https://rocm.docs.amd.com/en/latest/".to_string(),
                    ],
                });
            }
        }
    }

    None
}

/// Provides Intel-specific setup guidance.
fn get_intel_setup_guidance(gpu_info: &GpuInfo) -> Option<GpuSetupGuidance> {
    // Intel Arc support is still experimental in Ollama
    Some(GpuSetupGuidance {
        issue: format!("{} detected - Intel Arc GPU support is experimental", gpu_info.name),
        solution: "Intel Arc GPU support requires oneAPI and is experimental in Ollama".to_string(),
        commands: vec![
            "# Install Intel oneAPI Base Toolkit:".to_string(),
            "# https://www.intel.com/content/www/us/en/developer/tools/oneapi/base-toolkit-download.html".to_string(),
            "# Ensure SYCL runtime is available for Ollama".to_string(),
        ],
        links: vec![
            "https://github.com/ollama/ollama/blob/main/docs/gpu.md".to_string(),
            "https://www.intel.com/content/www/us/en/developer/tools/oneapi/base-toolkit.html".to_string(),
        ],
    })
}

/// Comprehensive GPU setup status for the gpu-setup command.
#[derive(Debug, Clone)]
pub struct GpuSetupStatus {
    /// Information about the detected GPU
    pub gpu_info: GpuInfo,
    /// Whether GPU acceleration appears to be working
    pub gpu_working: bool,
    /// Any issues detected with setup guidance
    pub guidance: Option<GpuSetupGuidance>,
    /// Models currently loaded and their GPU status
    pub loaded_models: Vec<LoadedModelInfo>,
    /// Real-time VRAM usage if available
    pub vram_usage: Option<GpuMemoryUsage>,
    /// AMD architecture if applicable
    pub amd_architecture: Option<AmdArchitecture>,
    /// Whether ROCm/HIP is installed (for AMD)
    pub rocm_installed: bool,
    /// NVIDIA driver version if applicable
    pub nvidia_driver: Option<String>,
}

/// Gets comprehensive GPU setup status for the gpu-setup command.
///
/// This function provides a complete overview of GPU setup status,
/// including detection, driver status, and any issues that need addressing.
///
/// # Returns
/// A `GpuSetupStatus` with all GPU setup information.
pub fn get_gpu_setup_status() -> GpuSetupStatus {
    let gpu_info = detect_gpu().unwrap_or_default();
    let loaded_models = get_ollama_loaded_models();
    let vram_usage = get_gpu_memory_usage(Some(&gpu_info));
    let guidance = get_gpu_setup_guidance(&gpu_info);

    // Determine if GPU is actually working
    let gpu_working = if gpu_info.gpu_type == GpuType::Cpu {
        false
    } else {
        // Check if any loaded model is using GPU
        loaded_models.iter().any(|m| {
            matches!(m.processor, ProcessorType::Gpu(_))
                || matches!(m.processor, ProcessorType::Mixed { gpu_percent, .. } if gpu_percent > 0)
        }) || guidance.is_none() // If no guidance needed, assume it's working
    };

    // AMD-specific info
    let amd_architecture = if gpu_info.gpu_type == GpuType::Amd {
        Some(detect_amd_architecture(&gpu_info.name))
    } else {
        None
    };

    let rocm_installed = if gpu_info.gpu_type == GpuType::Amd {
        check_rocm_installed()
    } else {
        false
    };

    // NVIDIA-specific info
    let nvidia_driver = if gpu_info.gpu_type == GpuType::Nvidia {
        get_nvidia_driver_version()
    } else {
        None
    };

    GpuSetupStatus {
        gpu_info,
        gpu_working,
        guidance,
        loaded_models,
        vram_usage,
        amd_architecture,
        rocm_installed,
        nvidia_driver,
    }
}

/// Generates a comprehensive GPU status report.
///
/// This function collects all GPU-related information for display
/// in the `rigrun status` command.
///
/// # Returns
/// A `GpuStatusReport` with all GPU information.
pub fn get_gpu_status_report() -> GpuStatusReport {
    let gpu_info = detect_gpu().unwrap_or_default();
    let vram_usage = get_gpu_memory_usage(Some(&gpu_info));
    let loaded_models = get_ollama_loaded_models();

    let mut warnings = Vec::new();

    // Check each loaded model for CPU fallback
    for model in &loaded_models {
        match &model.processor {
            ProcessorType::Cpu => {
                warnings.push(format!(
                    "Model '{}' is running on CPU - inference will be slow",
                    model.name
                ));
            }
            ProcessorType::Mixed { gpu_percent, cpu_percent } => {
                if *cpu_percent > 50 {
                    warnings.push(format!(
                        "Model '{}' is mostly on CPU ({}% CPU/{}% GPU) - performance degraded",
                        model.name, cpu_percent, gpu_percent
                    ));
                }
            }
            _ => {}
        }
    }

    // Check if VRAM is nearly full
    if let Some(ref usage) = vram_usage {
        if usage.usage_percent() > 90.0 {
            warnings.push(format!(
                "VRAM is nearly full ({:.1}% used) - may cause slowdowns",
                usage.usage_percent()
            ));
        }
    }

    GpuStatusReport {
        gpu_info,
        vram_usage,
        loaded_models,
        warnings,
    }
}

/// Detects the AMD GPU GFX version (e.g., gfx1100, gfx1200) for the given GPU.
///
/// This is used to determine if the GPU needs special handling like HSA override
/// or the ollama-for-amd fork.
fn detect_amd_gfx_version(gpu_info: &GpuInfo) -> Option<String> {
    // Infer GFX version from GPU name using architecture detection
    let arch = detect_amd_architecture(&gpu_info.name);
    match arch {
        AmdArchitecture::Rdna4 => Some("gfx1200".to_string()),
        AmdArchitecture::Rdna3 => {
            let name_lower = gpu_info.name.to_lowercase();
            if name_lower.contains("7600") {
                Some("gfx1102".to_string())
            } else {
                Some("gfx1100".to_string())
            }
        }
        AmdArchitecture::Rdna2 => {
            let name_lower = gpu_info.name.to_lowercase();
            if name_lower.contains("6600") || name_lower.contains("6500") || name_lower.contains("6400") {
                Some("gfx1032".to_string())
            } else {
                Some("gfx1030".to_string())
            }
        }
        AmdArchitecture::Rdna1 => Some("gfx1010".to_string()),
        AmdArchitecture::Vega => Some("gfx906".to_string()),
        AmdArchitecture::Unknown => None,
    }
}

/// Checks if nvidia-smi works properly (CUDA drivers installed and functional).
fn check_nvidia_cuda_functional() -> bool {
    let nvidia_smi_paths = if cfg!(target_os = "windows") {
        vec![
            "nvidia-smi",
            "C:\\Windows\\System32\\nvidia-smi.exe",
            "C:\\Program Files\\NVIDIA Corporation\\NVSMI\\nvidia-smi.exe",
        ]
    } else {
        vec!["nvidia-smi"]
    };

    for path in nvidia_smi_paths {
        if let Ok(output) = Command::new(path)
            .args(["--query-gpu=name", "--format=csv,noheader"])
            .output()
        {
            if output.status.success() && !output.stdout.is_empty() {
                return true;
            }
        }
    }
    false
}

/// Checks if HSA_OVERRIDE_GFX_VERSION environment variable is set.
fn check_hsa_override_set() -> bool {
    std::env::var("HSA_OVERRIDE_GFX_VERSION").is_ok()
}

/// Diagnoses why a model might be running on CPU when VRAM is sufficient.
///
/// Returns specific guidance based on the detected issue. This function checks:
/// 1. Whether the model is loaded via `ollama ps`
/// 2. If model is on CPU, checks if VRAM is sufficient for the model
/// 3. Determines GPU type (AMD vs NVIDIA) and diagnoses accordingly
///
/// For AMD GPUs:
/// - Checks if gfx1200 (RDNA4) which needs ollama-for-amd fork
/// - Checks if HSA_OVERRIDE_GFX_VERSION is set
/// - Checks if ROCm folder exists
///
/// For NVIDIA GPUs:
/// - Checks if nvidia-smi works
/// - Checks CUDA availability
///
/// # Arguments
/// * `model_name` - The name of the model to check
/// * `gpu_info` - Information about the detected GPU
///
/// # Returns
/// `Some(CpuFallbackDiagnosis)` if the model is on CPU with sufficient VRAM,
/// `None` otherwise (model not loaded, using GPU, or VRAM is insufficient).
///
/// # Example
/// ```rust,no_run
/// use rigrun::detect::{detect_gpu, diagnose_cpu_fallback};
///
/// let gpu = detect_gpu().unwrap_or_default();
/// if let Some(diagnosis) = diagnose_cpu_fallback("qwen2.5-coder:14b", &gpu) {
///     println!("Issue: {}", diagnosis.likely_cause);
///     for step in &diagnosis.fix_steps {
///         println!("  - {}", step);
///     }
/// }
/// ```
pub fn diagnose_cpu_fallback(
    model_name: &str,
    gpu_info: &GpuInfo,
) -> Option<CpuFallbackDiagnosis> {
    // Check if model is loaded via `ollama ps`
    let loaded_model = get_model_gpu_status(model_name)?;

    // Check if model is actually on CPU
    let model_on_cpu = match &loaded_model.processor {
        ProcessorType::Cpu => true,
        ProcessorType::Mixed { gpu_percent, .. } => *gpu_percent == 0,
        ProcessorType::Gpu(_) => false,
        ProcessorType::Unknown => false,
    };

    if !model_on_cpu {
        // Model is using GPU, no diagnosis needed
        return None;
    }

    // Check if VRAM is sufficient for the model
    let required_vram = estimate_model_vram(model_name);
    let has_sufficient_vram = match required_vram {
        Some(required) => gpu_info.vram_gb >= required,
        None => {
            // Can't estimate from model name, use loaded model size
            let model_size_gb = (loaded_model.size_bytes as f64 / 1_073_741_824.0).ceil() as u32;
            gpu_info.vram_gb >= model_size_gb
        }
    };

    if !has_sufficient_vram {
        // VRAM is insufficient, CPU fallback is expected - no diagnosis needed
        return None;
    }

    // We have sufficient VRAM but model is on CPU - diagnose the issue
    let (likely_cause, fix_steps) = match gpu_info.gpu_type {
        GpuType::Amd => diagnose_amd_cpu_fallback_cause(gpu_info),
        GpuType::Nvidia => diagnose_nvidia_cpu_fallback_cause(),
        GpuType::Cpu => {
            // No GPU detected at all
            (CpuFallbackCause::Unknown, vec![
                "No GPU detected on the system".to_string(),
                "Check if GPU drivers are installed correctly".to_string(),
            ])
        }
        GpuType::Intel | GpuType::AppleSilicon => {
            // Intel Arc and Apple Silicon have limited Ollama support
            (CpuFallbackCause::OllamaNoGpuSupport, vec![
                format!("{} GPU may have limited Ollama support", gpu_info.gpu_type),
                "Check Ollama documentation for GPU compatibility".to_string(),
            ])
        }
    };

    Some(CpuFallbackDiagnosis {
        has_sufficient_vram,
        model_on_cpu,
        likely_cause,
        fix_steps,
    })
}

/// Diagnoses AMD-specific CPU fallback issues.
fn diagnose_amd_cpu_fallback_cause(gpu_info: &GpuInfo) -> (CpuFallbackCause, Vec<String>) {
    let gfx_version = detect_amd_gfx_version(gpu_info).unwrap_or_else(|| "unknown".to_string());
    let arch = detect_amd_architecture(&gpu_info.name);

    // Check for RDNA 4 (gfx1200) which needs special Ollama fork
    if arch == AmdArchitecture::Rdna4 || gfx_version.starts_with("gfx12") {
        return (
            CpuFallbackCause::AmdNeedsCustomOllama { gfx_version: gfx_version.clone() },
            vec![
                format!("Your {} is RDNA 4 ({})", gpu_info.name, gfx_version),
                "RDNA 4 requires the 'ollama-for-amd' fork from GitHub".to_string(),
                "Download from: https://github.com/likelovewant/ollama-for-amd".to_string(),
                "After installing, restart Ollama and try again".to_string(),
            ],
        );
    }

    // Check if ROCm is installed
    if !check_rocm_installed() {
        return (
            CpuFallbackCause::AmdMissingRocm,
            vec![
                "ROCm/HIP runtime is not installed or not found".to_string(),
                "Ollama needs ROCm to use AMD GPUs".to_string(),
                if cfg!(target_os = "windows") {
                    "Download Ollama AMD build or install HIP SDK".to_string()
                } else {
                    "Install ROCm: https://rocm.docs.amd.com/en/latest/".to_string()
                },
                "After installing, restart Ollama".to_string(),
            ],
        );
    }

    // Check if HSA_OVERRIDE_GFX_VERSION might help
    if !check_hsa_override_set() {
        if let Some(suggested) = get_hsa_override_version(&gpu_info.name) {
            return (
                CpuFallbackCause::AmdNeedsHsaOverride { suggested_version: suggested.clone() },
                vec![
                    format!("Your GPU ({}) may need HSA override", gfx_version),
                    format!("Try setting: HSA_OVERRIDE_GFX_VERSION={}", suggested),
                    if cfg!(target_os = "windows") {
                        format!("Run: setx HSA_OVERRIDE_GFX_VERSION {}", suggested)
                    } else {
                        format!("Add to ~/.bashrc: export HSA_OVERRIDE_GFX_VERSION={}", suggested)
                    },
                    "Then restart Ollama and try again".to_string(),
                ],
            );
        }
    }

    // Generic Ollama GPU support issue
    (
        CpuFallbackCause::OllamaNoGpuSupport,
        vec![
            "Ollama may not have GPU support enabled".to_string(),
            "Try reinstalling Ollama with AMD GPU support".to_string(),
            "Check that ROCm is properly configured".to_string(),
            format!("GPU: {} ({})", gpu_info.name, gfx_version),
        ],
    )
}

/// Diagnoses NVIDIA-specific CPU fallback issues.
fn diagnose_nvidia_cpu_fallback_cause() -> (CpuFallbackCause, Vec<String>) {
    if !check_nvidia_cuda_functional() {
        return (
            CpuFallbackCause::NvidiaMissingCuda,
            vec![
                "NVIDIA CUDA drivers may be missing or outdated".to_string(),
                "Install or update NVIDIA drivers from: https://www.nvidia.com/drivers".to_string(),
                "Make sure nvidia-smi works: nvidia-smi".to_string(),
                "After updating drivers, restart Ollama".to_string(),
            ],
        );
    }

    // CUDA is available but still on CPU - could be Ollama issue
    (
        CpuFallbackCause::OllamaNoGpuSupport,
        vec![
            "CUDA drivers are installed but Ollama is not using the GPU".to_string(),
            "Try restarting Ollama: ollama stop && ollama serve".to_string(),
            "Check Ollama logs for GPU detection errors".to_string(),
            "Try reinstalling Ollama if the issue persists".to_string(),
        ],
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_gpu_type_display() {
        assert_eq!(format!("{}", GpuType::Nvidia), "NVIDIA");
        assert_eq!(format!("{}", GpuType::Amd), "AMD");
        assert_eq!(format!("{}", GpuType::AppleSilicon), "Apple Silicon");
        assert_eq!(format!("{}", GpuType::Intel), "Intel Arc");
        assert_eq!(format!("{}", GpuType::Cpu), "CPU");
    }

    #[test]
    fn test_gpu_info_default() {
        let info = GpuInfo::default();
        assert_eq!(info.name, "CPU Only");
        assert_eq!(info.vram_gb, 0);
        assert!(info.driver.is_none());
        assert_eq!(info.gpu_type, GpuType::Cpu);
    }

    #[test]
    fn test_gpu_info_display() {
        let info = GpuInfo {
            name: "NVIDIA RTX 4090".to_string(),
            vram_gb: 24,
            driver: Some("535.104.05".to_string()),
            gpu_type: GpuType::Nvidia,
        };
        let display = format!("{}", info);
        assert!(display.contains("NVIDIA RTX 4090"));
        assert!(display.contains("24GB"));
        assert!(display.contains("535.104.05"));
    }

    #[test]
    fn test_recommend_model_low_vram() {
        assert_eq!(recommend_model(0), "qwen2.5-coder:3b");
        assert_eq!(recommend_model(4), "qwen2.5-coder:3b");
        assert_eq!(recommend_model(5), "qwen2.5-coder:3b");
    }

    #[test]
    fn test_recommend_model_medium_vram() {
        // 6-9GB: qwen2.5-coder:7b
        assert_eq!(recommend_model(6), "qwen2.5-coder:7b");
        assert_eq!(recommend_model(7), "qwen2.5-coder:7b");
        assert_eq!(recommend_model(8), "qwen2.5-coder:7b");
        assert_eq!(recommend_model(9), "qwen2.5-coder:7b");
        // 10-17GB: qwen2.5-coder:14b
        assert_eq!(recommend_model(10), "qwen2.5-coder:14b");
        assert_eq!(recommend_model(12), "qwen2.5-coder:14b");
        assert_eq!(recommend_model(16), "qwen2.5-coder:14b");
        assert_eq!(recommend_model(17), "qwen2.5-coder:14b");
    }

    #[test]
    fn test_recommend_model_high_vram() {
        // 18-26GB: qwen2.5-coder:32b
        assert_eq!(recommend_model(18), "qwen2.5-coder:32b");
        assert_eq!(recommend_model(20), "qwen2.5-coder:32b");
        assert_eq!(recommend_model(24), "qwen2.5-coder:32b");
        assert_eq!(recommend_model(26), "qwen2.5-coder:32b");
    }

    #[test]
    fn test_recommend_model_very_high_vram() {
        // 27-47GB: qwen3-coder:30b
        assert_eq!(recommend_model(27), "qwen3-coder:30b");
        assert_eq!(recommend_model(32), "qwen3-coder:30b");
        assert_eq!(recommend_model(47), "qwen3-coder:30b");
    }

    #[test]
    fn test_recommend_model_extreme_vram() {
        // 48GB+: qwen3-coder:30b
        assert_eq!(recommend_model(48), "qwen3-coder:30b");
        assert_eq!(recommend_model(80), "qwen3-coder:30b");
        assert_eq!(recommend_model(128), "qwen3-coder:30b");
    }

    #[test]
    fn test_recommend_model_alt() {
        assert_eq!(recommend_model_alt(16), Some("qwen2.5-coder:32b".to_string()));
        assert_eq!(recommend_model_alt(8), None);
        assert_eq!(recommend_model_alt(24), None);
    }

    #[test]
    fn test_gpu_detect_error_display() {
        let err = GpuDetectError::CommandFailed("test".to_string());
        assert!(format!("{}", err).contains("Command failed"));

        let err = GpuDetectError::ParseError("test".to_string());
        assert!(format!("{}", err).contains("Parse error"));

        let err = GpuDetectError::NoGpuFound;
        assert!(format!("{}", err).contains("No GPU found"));
    }

    #[test]
    fn test_detect_gpu_returns_ok() {
        // This test verifies that detect_gpu always returns Ok
        // even when no GPU is found (falls back to CPU)
        let result = detect_gpu();
        assert!(result.is_ok());
    }

    #[test]
    fn test_infer_amd_vram_rx_9070_xt() {
        // Test RX 9070 XT (16GB)
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 9070 XT"), Some(16));
        assert_eq!(infer_amd_vram_from_model("Radeon RX 9070 XT"), Some(16));
        assert_eq!(infer_amd_vram_from_model("RX 9070 XT"), Some(16));
    }

    #[test]
    fn test_infer_amd_vram_rx_9070() {
        // Test RX 9070 (12GB)
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 9070"), Some(12));
        assert_eq!(infer_amd_vram_from_model("RX 9070"), Some(12));
    }

    #[test]
    fn test_infer_amd_vram_rx_7900_series() {
        // Test RX 7900 series
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 7900 XTX"), Some(24));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 7900 XT"), Some(20));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 7900 GRE"), Some(20));
    }

    #[test]
    fn test_infer_amd_vram_rx_7800_xt() {
        // Test RX 7800 XT (16GB)
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 7800 XT"), Some(16));
    }

    #[test]
    fn test_infer_amd_vram_rx_6900_xt() {
        // Test RX 6900 XT (16GB)
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 6900 XT"), Some(16));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon RX 6950 XT"), Some(16));
    }

    #[test]
    fn test_infer_amd_vram_radeon_vii() {
        // Test Radeon VII (16GB)
        assert_eq!(infer_amd_vram_from_model("AMD Radeon VII"), Some(16));
        assert_eq!(infer_amd_vram_from_model("Radeon 7"), Some(16));
    }

    #[test]
    fn test_infer_amd_vram_pro_series() {
        // Test Pro series workstation cards
        assert_eq!(infer_amd_vram_from_model("AMD Radeon Pro W7900"), Some(48));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon Pro W7800"), Some(32));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon Pro W7700"), Some(16));
        assert_eq!(infer_amd_vram_from_model("AMD Radeon Pro W6800"), Some(32));
    }

    #[test]
    fn test_infer_amd_vram_unknown_model() {
        // Test unknown model returns None
        assert_eq!(infer_amd_vram_from_model("AMD Unknown GPU"), None);
        assert_eq!(infer_amd_vram_from_model("Some Random GPU"), None);
    }

    #[test]
    fn test_infer_amd_vram_case_insensitive() {
        // Test case insensitivity
        assert_eq!(infer_amd_vram_from_model("amd radeon rx 9070 xt"), Some(16));
        assert_eq!(infer_amd_vram_from_model("AMD RADEON RX 9070 XT"), Some(16));
        assert_eq!(infer_amd_vram_from_model("aMd RaDeOn Rx 9070 xT"), Some(16));
    }

    #[test]
    fn test_processor_type_display() {
        assert_eq!(format!("{}", ProcessorType::Gpu(100)), "100% GPU");
        assert_eq!(format!("{}", ProcessorType::Cpu), "100% CPU");
        assert_eq!(
            format!("{}", ProcessorType::Mixed { gpu_percent: 50, cpu_percent: 50 }),
            "50% GPU/50% CPU"
        );
        assert_eq!(format!("{}", ProcessorType::Unknown), "Unknown");
    }

    #[test]
    fn test_parse_size_to_bytes() {
        assert_eq!(parse_size_to_bytes("9.0 GB"), 9663676416); // 9 * 1024^3
        assert_eq!(parse_size_to_bytes("512 MB"), 536870912); // 512 * 1024^2
        assert_eq!(parse_size_to_bytes("1024 KB"), 1048576); // 1024 * 1024
        assert_eq!(parse_size_to_bytes("1000"), 1000);
    }

    #[test]
    fn test_parse_processor() {
        assert_eq!(parse_processor(&["100%", "GPU"]), ProcessorType::Gpu(100));
        assert_eq!(parse_processor(&["100%", "CPU"]), ProcessorType::Cpu);
        assert_eq!(
            parse_processor(&["50%", "GPU/50%", "CPU"]),
            ProcessorType::Mixed { gpu_percent: 50, cpu_percent: 50 }
        );
        assert_eq!(parse_processor(&[]), ProcessorType::Unknown);
    }

    #[test]
    fn test_estimate_model_vram() {
        // Test various model sizes
        assert!(estimate_model_vram("qwen2.5-coder:3b").is_some());
        assert!(estimate_model_vram("qwen2.5-coder:7b").is_some());
        assert!(estimate_model_vram("qwen2.5-coder:14b").is_some());
        assert!(estimate_model_vram("qwen2.5-coder:32b").is_some());

        // Unknown model
        assert!(estimate_model_vram("unknown-model").is_none());

        // Larger models should require more VRAM
        let vram_7b = estimate_model_vram("qwen2.5-coder:7b").unwrap();
        let vram_14b = estimate_model_vram("qwen2.5-coder:14b").unwrap();
        let vram_32b = estimate_model_vram("qwen2.5-coder:32b").unwrap();
        assert!(vram_7b < vram_14b);
        assert!(vram_14b < vram_32b);
    }

    #[test]
    fn test_will_model_fit_in_vram() {
        // 7B model should fit in 16GB
        assert!(will_model_fit_in_vram("qwen2.5-coder:7b", 16).unwrap());

        // 32B model should NOT fit in 8GB
        assert!(!will_model_fit_in_vram("qwen2.5-coder:32b", 8).unwrap());

        // Unknown model returns error
        assert!(will_model_fit_in_vram("unknown-model", 16).is_err());
    }

    #[test]
    fn test_check_gpu_utilization_cpu_fallback() {
        let gpu_info = GpuInfo {
            name: "NVIDIA RTX 3060".to_string(),
            vram_gb: 12,
            driver: Some("535.0".to_string()),
            gpu_type: GpuType::Nvidia,
        };

        // 32B model is too large for 12GB - should suggest smaller model
        let status = check_gpu_utilization("qwen2.5-coder:32b", &gpu_info);
        assert!(status.warning.is_some());
        assert!(status.suggested_model.is_some());
    }

    #[test]
    fn test_check_gpu_utilization_model_fits() {
        let gpu_info = GpuInfo {
            name: "NVIDIA RTX 4090".to_string(),
            vram_gb: 24,
            driver: Some("535.0".to_string()),
            gpu_type: GpuType::Nvidia,
        };

        // Use a model name unlikely to be loaded to test estimation logic
        // This tests the "model not loaded" branch which estimates based on VRAM
        let status = check_gpu_utilization("test-model:7b", &gpu_info);
        // Model not loaded, so it estimates. 7B ~= 6GB, fits in 24GB.
        // If model IS loaded (unlikely), it checks actual processor usage.
        // Either way, 7B should fit in 24GB without warning.
        if !status.using_gpu && status.warning.is_some() {
            // Model is loaded on CPU - this can happen if Ollama has a model running
            // Skip the assertion in this case as it depends on Ollama state
            return;
        }
        assert!(status.warning.is_none());
        assert!(status.suggested_model.is_none());
    }

    #[test]
    fn test_gpu_memory_usage_methods() {
        let usage = GpuMemoryUsage {
            total_mb: 16384, // 16GB
            used_mb: 8192,   // 8GB
            free_mb: 8192,   // 8GB
            gpu_utilization: Some(75),
        };

        assert_eq!(usage.total_gb(), 16);
        assert_eq!(usage.used_gb(), 8);
        assert_eq!(usage.free_gb(), 8);
        assert!((usage.usage_percent() - 50.0).abs() < 0.01);
    }

    #[test]
    fn test_gpu_memory_usage_edge_cases() {
        // Test with zero values
        let zero_usage = GpuMemoryUsage {
            total_mb: 0,
            used_mb: 0,
            free_mb: 0,
            gpu_utilization: None,
        };
        assert_eq!(zero_usage.usage_percent(), 0.0);
        assert_eq!(zero_usage.total_gb(), 0);

        // Test with small values (rounding)
        let small_usage = GpuMemoryUsage {
            total_mb: 512, // 0.5GB
            used_mb: 256,  // 0.25GB
            free_mb: 256,
            gpu_utilization: Some(50),
        };
        // 512MB rounds to 0GB or 1GB depending on rounding
        assert!(small_usage.total_gb() <= 1);
    }

    #[test]
    fn test_format_cpu_fallback_warning() {
        let lines = format_cpu_fallback_warning(
            "qwen2.5-coder:32b",
            12,
            Some(20),
            Some("qwen2.5-coder:14b"),
        );

        assert!(lines.len() >= 3);
        assert!(lines[0].contains("qwen2.5-coder:32b"));
        assert!(lines[0].contains("CPU"));
        assert!(lines[1].contains("12GB"));
        assert!(lines[1].contains("20GB"));
        assert!(lines[2].contains("qwen2.5-coder:14b"));
    }

    #[test]
    fn test_format_cpu_fallback_warning_minimal() {
        let lines = format_cpu_fallback_warning(
            "model",
            8,
            None,
            None,
        );

        assert_eq!(lines.len(), 1);
        assert!(lines[0].contains("model"));
    }

    #[test]
    fn test_post_load_gpu_check_not_loaded() {
        let gpu_info = GpuInfo {
            name: "NVIDIA RTX 4090".to_string(),
            vram_gb: 24,
            driver: Some("535.0".to_string()),
            gpu_type: GpuType::Nvidia,
        };

        // This test assumes no model is currently loaded
        // The check should return model_loaded = false
        let check = check_loaded_model_gpu_usage("nonexistent-model:latest", &gpu_info);
        assert!(!check.model_loaded);
        assert!(!check.using_gpu);
        assert!(check.processor.is_none());
        assert!(check.model_size.is_none());
    }

    #[test]
    fn test_gpu_status_report_structure() {
        // Just verify the function doesn't panic and returns valid structure
        let report = get_gpu_status_report();
        // The GPU info should be valid (even if it's CPU mode)
        assert!(!report.gpu_info.name.is_empty());
        // Warnings should be a valid vector (just verify it exists)
        let _ = report.warnings.len();
    }

    #[test]
    fn test_cpu_fallback_cause_display() {
        // Test display formatting for all CpuFallbackCause variants
        assert!(format!("{}", CpuFallbackCause::AmdNeedsCustomOllama {
            gfx_version: "gfx1200".to_string()
        }).contains("gfx1200"));

        assert!(format!("{}", CpuFallbackCause::AmdMissingRocm)
            .contains("ROCm"));

        assert!(format!("{}", CpuFallbackCause::AmdNeedsHsaOverride {
            suggested_version: "11.0.0".to_string()
        }).contains("11.0.0"));

        assert!(format!("{}", CpuFallbackCause::NvidiaMissingCuda)
            .contains("CUDA"));

        assert!(format!("{}", CpuFallbackCause::OllamaNoGpuSupport)
            .contains("Ollama"));

        assert!(format!("{}", CpuFallbackCause::Unknown)
            .contains("Unknown"));
    }

    #[test]
    fn test_detect_amd_gfx_version_rdna4() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 9070 XT".to_string(),
            vram_gb: 16,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, Some("gfx1200".to_string()));
    }

    #[test]
    fn test_detect_amd_gfx_version_rdna3() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 7900 XTX".to_string(),
            vram_gb: 24,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, Some("gfx1100".to_string()));
    }

    #[test]
    fn test_detect_amd_gfx_version_rdna3_7600() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 7600".to_string(),
            vram_gb: 8,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, Some("gfx1102".to_string()));
    }

    #[test]
    fn test_detect_amd_gfx_version_rdna2() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 6900 XT".to_string(),
            vram_gb: 16,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, Some("gfx1030".to_string()));
    }

    #[test]
    fn test_detect_amd_gfx_version_rdna2_6600() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 6600 XT".to_string(),
            vram_gb: 8,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, Some("gfx1032".to_string()));
    }

    #[test]
    fn test_detect_amd_gfx_version_unknown() {
        let gpu = GpuInfo {
            name: "AMD Unknown GPU".to_string(),
            vram_gb: 8,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let gfx = detect_amd_gfx_version(&gpu);
        assert_eq!(gfx, None);
    }

    #[test]
    fn test_diagnose_amd_cpu_fallback_cause_rdna4() {
        let gpu = GpuInfo {
            name: "AMD Radeon RX 9070 XT".to_string(),
            vram_gb: 16,
            driver: None,
            gpu_type: GpuType::Amd,
        };
        let (cause, steps) = diagnose_amd_cpu_fallback_cause(&gpu);

        // RDNA 4 should suggest the ollama-for-amd fork
        match cause {
            CpuFallbackCause::AmdNeedsCustomOllama { gfx_version } => {
                assert!(gfx_version.starts_with("gfx12"));
            }
            _ => panic!("Expected AmdNeedsCustomOllama for RDNA 4 GPU"),
        }

        // Should have fix steps
        assert!(!steps.is_empty());
        // Should mention ollama-for-amd
        assert!(steps.iter().any(|s| s.contains("ollama-for-amd")));
    }

    #[test]
    fn test_diagnose_nvidia_cpu_fallback_cause() {
        // Test the NVIDIA diagnosis function
        let (cause, steps) = diagnose_nvidia_cpu_fallback_cause();

        // Should return one of the expected causes
        assert!(matches!(
            cause,
            CpuFallbackCause::NvidiaMissingCuda | CpuFallbackCause::OllamaNoGpuSupport
        ));

        // Should have fix steps
        assert!(!steps.is_empty());
    }

    #[test]
    fn test_diagnose_cpu_fallback_model_not_loaded() {
        let gpu = GpuInfo {
            name: "NVIDIA RTX 4090".to_string(),
            vram_gb: 24,
            driver: Some("535.0".to_string()),
            gpu_type: GpuType::Nvidia,
        };

        // For a model that's not loaded, should return None
        let diagnosis = diagnose_cpu_fallback("nonexistent-model-xyz:latest", &gpu);
        assert!(diagnosis.is_none());
    }

    #[test]
    fn test_cpu_fallback_diagnosis_struct() {
        // Test that CpuFallbackDiagnosis can be constructed
        let diagnosis = CpuFallbackDiagnosis {
            has_sufficient_vram: true,
            model_on_cpu: true,
            likely_cause: CpuFallbackCause::AmdNeedsCustomOllama {
                gfx_version: "gfx1200".to_string(),
            },
            fix_steps: vec![
                "Step 1".to_string(),
                "Step 2".to_string(),
            ],
        };

        assert!(diagnosis.has_sufficient_vram);
        assert!(diagnosis.model_on_cpu);
        assert_eq!(diagnosis.fix_steps.len(), 2);
    }
}

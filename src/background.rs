// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

// Background server implementation for rigrun

use anyhow::{Context, Result};
use std::path::PathBuf;

use crate::colors::*;
use crate::{is_rigrun_process, kill_process, find_process_on_port, load_config, DEFAULT_PORT};

#[cfg(target_os = "windows")]
pub fn find_all_rigrun_processes() -> Vec<u32> {
    let mut pids = Vec::new();

    if let Ok(output) = std::process::Command::new("tasklist")
        .args(["/FO", "CSV", "/NH"])
        .output()
    {
        let output_str = String::from_utf8_lossy(&output.stdout);
        for line in output_str.lines() {
            if line.contains("rigrun.exe") {
                // CSV format: "name","PID","Session","Mem Usage"
                let parts: Vec<&str> = line.split(',').collect();
                if parts.len() >= 2 {
                    let pid_str = parts[1].trim_matches('"').trim();
                    if let Ok(pid) = pid_str.parse::<u32>() {
                        pids.push(pid);
                    }
                }
            }
        }
    }

    pids
}

#[cfg(not(target_os = "windows"))]
pub fn find_all_rigrun_processes() -> Vec<u32> {
    let mut pids = Vec::new();

    if let Ok(output) = std::process::Command::new("pgrep")
        .args(["rigrun"])
        .output()
    {
        let output_str = String::from_utf8_lossy(&output.stdout);
        for line in output_str.lines() {
            if let Ok(pid) = line.trim().parse::<u32>() {
                pids.push(pid);
            }
        }
    }

    pids
}

#[cfg(target_os = "windows")]
fn get_startup_folder() -> Result<PathBuf> {
    // Get the Windows Startup folder path
    let startup = dirs::data_local_dir()
        .context("Could not find local data directory")?
        .join("Microsoft")
        .join("Windows")
        .join("Start Menu")
        .join("Programs")
        .join("Startup");

    Ok(startup)
}

#[cfg(target_os = "windows")]
fn create_startup_shortcut() -> Result<()> {
    use std::process::Command;

    let startup_folder = get_startup_folder()?;
    let shortcut_path = startup_folder.join("RigRun.lnk");

    // Get the current executable path
    let exe_path = std::env::current_exe()?;

    // Create a PowerShell script to create the shortcut
    let ps_script = format!(
        r#"
        $WshShell = New-Object -ComObject WScript.Shell
        $Shortcut = $WshShell.CreateShortcut('{}')
        $Shortcut.TargetPath = '{}'
        $Shortcut.Arguments = ''
        $Shortcut.WindowStyle = 7
        $Shortcut.Description = 'RigRun LLM Server'
        $Shortcut.Save()
        "#,
        shortcut_path.display().to_string().replace('\\', "\\\\"),
        exe_path.display().to_string().replace('\\', "\\\\")
    );

    let status = Command::new("powershell")
        .args(["-NoProfile", "-Command", &ps_script])
        .status()?;

    if !status.success() {
        anyhow::bail!("Failed to create startup shortcut");
    }

    Ok(())
}

pub fn handle_background_server() -> Result<()> {
    println!();
    println!("{CYAN}{BOLD}=== Run as Background Server ==={RESET}");
    println!();
    println!("{BOLD}Background mode runs rigrun as a detached process:{RESET}");
    println!("  - Server stays running after terminal closes");
    println!("  - Accessible at {CYAN}http://localhost:8787{RESET}");
    println!("  - Can be stopped with {CYAN}rigrun stop{RESET}");
    println!();

    #[cfg(target_os = "windows")]
    {
        use inquire::Select;

        let options = vec![
            "Run in background now",
            "Install as Windows Service (coming soon)",
            "Create startup shortcut",
            "Cancel",
        ];

        let answer = Select::new("Choose an option:", options)
            .prompt()
            .context("Failed to get user selection")?;

        match answer {
            "Run in background now" => {
                println!();
                println!("{CYAN}[↻]{RESET} Starting rigrun in background...");

                // Get the current executable path
                let exe_path = std::env::current_exe()?;

                // Spawn rigrun as a detached process
                use std::os::windows::process::CommandExt;
                const CREATE_NO_WINDOW: u32 = 0x08000000;
                const DETACHED_PROCESS: u32 = 0x00000008;

                let child = std::process::Command::new(&exe_path)
                    .creation_flags(CREATE_NO_WINDOW | DETACHED_PROCESS)
                    .spawn()?;

                let pid = child.id();

                // Wait a moment for the server to start
                std::thread::sleep(std::time::Duration::from_millis(1000));

                // Check if the process is still running
                if is_rigrun_process(pid) {
                    println!("{GREEN}[✓]{RESET} Server running in background (PID: {BOLD}{pid}{RESET})");
                    println!();
                    println!("Access at: {CYAN}http://localhost:8787{RESET}");
                    println!();
                    println!("Stop with:");
                    println!("  {CYAN}rigrun stop{RESET}");
                    println!("  {DIM}or{RESET}");
                    println!("  {CYAN}taskkill /PID {pid}{RESET}");
                } else {
                    println!("{RED}[✗]{RESET} Failed to start background server");
                    println!();
                    println!("Try running normally with: {CYAN}rigrun{RESET}");
                }
            }
            "Install as Windows Service (coming soon)" => {
                println!();
                println!("{CYAN}ℹ{RESET} {BOLD}Windows Service installation{RESET}");
                println!();
                println!("This feature will be available in a future update.");
                println!();
                println!("What it will do:");
                println!("  - Install rigrun as a Windows Service");
                println!("  - Starts automatically on boot");
                println!("  - Runs as SYSTEM user");
                println!("  - Manageable via Windows Services");
                println!();
                println!("For now, use '{CYAN}Create startup shortcut{RESET}' for auto-start.");
            }
            "Create startup shortcut" => {
                println!();
                println!("{CYAN}[↻]{RESET} Creating startup shortcut...");

                match create_startup_shortcut() {
                    Ok(_) => {
                        let startup_folder = get_startup_folder()?;
                        println!("{GREEN}[✓]{RESET} Startup shortcut created successfully!");
                        println!();
                        println!("Location: {BOLD}{}{RESET}", startup_folder.join("RigRun.lnk").display());
                        println!();
                        println!("RigRun will now:");
                        println!("  - Start automatically when you log in");
                        println!("  - Run minimized in the background");
                        println!("  - Be accessible at {CYAN}http://localhost:8787{RESET}");
                        println!();
                        println!("To remove: Delete the shortcut from the Startup folder");
                    }
                    Err(e) => {
                        println!("{RED}[✗]{RESET} Failed to create startup shortcut: {}", e);
                        println!();
                        println!("Manual steps:");
                        let exe_path = std::env::current_exe()?;
                        let startup_folder = get_startup_folder()?;
                        println!("  1. Create a shortcut to: {BOLD}{}{RESET}", exe_path.display());
                        println!("  2. Place it in: {BOLD}{}{RESET}", startup_folder.display());
                        println!("  3. Set it to run minimized");
                    }
                }
            }
            "Cancel" => {
                println!();
                println!("{YELLOW}⚠{RESET} Cancelled.");
            }
            _ => {}
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        use inquire::Select;

        let options = vec![
            "Run in background now",
            "Cancel",
        ];

        let answer = Select::new("Choose an option:", options)
            .prompt()
            .context("Failed to get user selection")?;

        match answer {
            "Run in background now" => {
                println!();
                println!("{CYAN}[↻]{RESET} Starting rigrun in background...");

                // Get the current executable path
                let exe_path = std::env::current_exe()?;

                // Spawn rigrun as a detached process
                let child = std::process::Command::new(&exe_path)
                    .stdin(std::process::Stdio::null())
                    .stdout(std::process::Stdio::null())
                    .stderr(std::process::Stdio::null())
                    .spawn()?;

                let pid = child.id();

                // Wait a moment for the server to start
                std::thread::sleep(std::time::Duration::from_millis(1000));

                // Check if the process is still running
                if is_rigrun_process(pid) {
                    println!("{GREEN}[✓]{RESET} Server running in background (PID: {BOLD}{pid}{RESET})");
                    println!();
                    println!("Access at: {CYAN}http://localhost:8787{RESET}");
                    println!();
                    println!("Stop with:");
                    println!("  {CYAN}rigrun stop{RESET}");
                    println!("  {DIM}or{RESET}");
                    println!("  {CYAN}kill {pid}{RESET}");
                } else {
                    println!("{RED}[✗]{RESET} Failed to start background server");
                    println!();
                    println!("Try running normally with: {CYAN}rigrun{RESET}");
                }
            }
            "Cancel" => {
                println!();
                println!("{YELLOW}⚠{RESET} Cancelled.");
            }
            _ => {}
        }
    }

    println!();
    Ok(())
}

pub fn handle_stop_server() -> Result<()> {
    println!();
    println!("{CYAN}[↻]{RESET} Searching for running rigrun processes...");

    let config = load_config()?;
    let port = config.port.unwrap_or(DEFAULT_PORT);

    // First, try to find process by port
    if let Some(pid) = find_process_on_port(port) {
        if is_rigrun_process(pid) {
            println!("{GREEN}[✓]{RESET} Found rigrun server on port {port} (PID: {pid})");
            println!("{CYAN}[↻]{RESET} Stopping server...");

            match kill_process(pid) {
                Ok(_) => {
                    println!("{GREEN}[✓]{RESET} Server stopped successfully");
                    println!();
                    return Ok(());
                }
                Err(e) => {
                    println!("{RED}[✗]{RESET} Failed to stop server: {}", e);
                }
            }
        }
    }

    // If not found by port, search for all rigrun processes
    let pids = find_all_rigrun_processes();

    if pids.is_empty() {
        println!("{YELLOW}⚠{RESET} No rigrun server running");
        println!();
        println!("Start one with: {CYAN}rigrun{RESET}");
        println!();
        return Ok(());
    }

    // Filter out the current process
    let current_pid = std::process::id();
    let other_pids: Vec<u32> = pids.into_iter().filter(|&pid| pid != current_pid).collect();

    if other_pids.is_empty() {
        println!("{YELLOW}⚠{RESET} No rigrun server running");
        println!();
        return Ok(());
    }

    println!("{GREEN}[✓]{RESET} Found {} rigrun process(es)", other_pids.len());
    println!();

    for pid in other_pids {
        println!("{CYAN}[↻]{RESET} Stopping process {pid}...");
        match kill_process(pid) {
            Ok(_) => println!("{GREEN}[✓]{RESET} Stopped process {pid}"),
            Err(e) => println!("{RED}[✗]{RESET} Failed to stop process {pid}: {}", e),
        }
    }

    println!();
    Ok(())
}

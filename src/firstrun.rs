use anyhow::Result;
use inquire::{Select, Text};
use super::{Config, save_config, interactive_chat, handle_cli_examples};

mod colors {
    pub const RESET: &str = "\x1b[0m";
    pub const BOLD: &str = "\x1b[1m";
    pub const DIM: &str = "\x1b[2m";
    pub const RED: &str = "\x1b[31m";
    pub const GREEN: &str = "\x1b[32m";
    pub const YELLOW: &str = "\x1b[33m";
    pub const CYAN: &str = "\x1b[36m";
    pub const WHITE: &str = "\x1b[37m";
    pub const BRIGHT_CYAN: &str = "\x1b[96m";
}

use colors::*;

/// Clear the terminal screen
fn clear_screen() {
    print!("\x1B[2J\x1B[1;1H");
    std::io::Write::flush(&mut std::io::stdout()).ok();
}

pub fn show_first_run_menu(config: &mut Config) -> Result<()> {
    // Start with a fresh screen
    println!();
    // Box width: 62 chars total (60 inner + 2 borders)
    // Text: "Welcome to RigRun - Your Local-First LLM Router" = 47 chars
    // Padding: (60-47)/2 = 6.5 -> 6 left, 7 right
    println!("{BRIGHT_CYAN}{BOLD}╔══════════════════════════════════════════════════════════╗{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}║                                                          ║{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}║     {WHITE}Welcome to RigRun - Your Local-First LLM Router{BRIGHT_CYAN}      ║{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}║                                                          ║{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}╚══════════════════════════════════════════════════════════╝{RESET}");
    println!();

    // Check if OpenRouter API key is not set - offer setup first
    if config.openrouter_key.is_none() {
        prompt_openrouter_setup(config)?;
        clear_screen();
    }

    println!();
    println!("{WHITE}Setup complete! What would you like to do next?{RESET}");
    println!();

    loop {
        let options = vec![
            "💬 Start chatting now",
            "🔧 Set up my IDE (Cursor/VS Code/etc)",
            "📋 Learn CLI commands",
            "🚀 Run as background server",
            "❓ Learn more about these options",
            "✨ All done - Continue running server",
        ];

        let selection = Select::new("Choose an option:", options)
            .with_help_message("↑↓ to move, enter to select")
            .prompt()?;

        match selection {
            "💬 Start chatting now" => {
                clear_screen();
                println!();
                println!("{GREEN}[✓]{RESET} Starting interactive chat...");
                println!("{DIM}(The server will continue running in the background){RESET}");
                println!();

                // Mark first run as complete before entering chat
                config.first_run_complete = true;
                save_config(config)?;

                // Start chat session
                if let Err(e) = interactive_chat(None) {
                    println!("{YELLOW}[!]{RESET} Chat error: {}", e);
                }

                clear_screen();
                println!();
                println!("{GREEN}[✓]{RESET} Returning to menu...");
                println!();
            }
            "🔧 Set up my IDE (Cursor/VS Code/etc)" => {
                clear_screen();
                handle_ide_setup()?;
                clear_screen();
            }
            "📋 Learn CLI commands" => {
                clear_screen();
                if let Err(e) = handle_cli_examples() {
                    println!("{YELLOW}[!]{RESET} Error showing examples: {}", e);
                }
                clear_screen();
            }
            "🚀 Run as background server" => {
                clear_screen();
                handle_background_server()?;
                clear_screen();
            }
            "❓ Learn more about these options" => {
                clear_screen();
                handle_learn_more()?;
                clear_screen();
            }
            "✨ All done - Continue running server" => {
                config.first_run_complete = true;
                save_config(config)?;
                // Don't show any message - screen will be cleared by main.rs
                break;
            }
            _ => {}
        }
    }

    Ok(())
}

/// Prompts the user to set up an OpenRouter API key for cloud fallback
fn prompt_openrouter_setup(config: &mut Config) -> Result<()> {
    println!();
    println!("{CYAN}{BOLD}Cloud Fallback Setup{RESET}");
    println!();
    println!("{WHITE}RigRun works best with a cloud fallback for complex queries.{RESET}");
    println!("{DIM}Local models handle most requests (free!), but an API key enables{RESET}");
    println!("{DIM}cloud routing when needed for harder tasks.{RESET}");
    println!();

    let options = vec![
        "Yes, let's do it",
        "No, local-only for now",
    ];

    let selection = Select::new("Would you like to set up cloud access now?", options)
        .with_help_message("↑↓ to move, enter to select")
        .prompt()?;

    match selection {
        "Yes, let's do it" => {
            println!();
            println!("{CYAN}[...]{RESET} Opening OpenRouter... Create a free account and generate an API key.");
            println!();

            // Open the browser to OpenRouter keys page
            let url = "https://openrouter.ai/keys";
            let open_result = open_browser(url);

            if open_result.is_err() {
                println!("{YELLOW}[!]{RESET} Could not open browser automatically.");
                println!("    Please visit: {CYAN}{url}{RESET}");
                println!();
            }

            // Prompt for the API key
            let api_key = Text::new("Paste your API key here:")
                .with_help_message("Press Enter after pasting your key (OpenRouter keys start with 'sk-or-')")
                .with_validator(|input: &str| {
                    let input = input.trim();
                    if input.is_empty() {
                        return Ok(inquire::validator::Validation::Invalid(
                            "API key cannot be empty. Enter a key or press Esc to skip.".into()
                        ));
                    }

                    // Check for common wrong key formats and warn
                    if input.starts_with("sk-ant-") {
                        return Ok(inquire::validator::Validation::Invalid(
                            "This looks like an Anthropic API key (sk-ant-...). OpenRouter keys start with 'sk-or-'. Get one at https://openrouter.ai/keys".into()
                        ));
                    }

                    if input.starts_with("sk-") && !input.starts_with("sk-or-") {
                        return Ok(inquire::validator::Validation::Invalid(
                            "This looks like an OpenAI API key (sk-...). OpenRouter keys start with 'sk-or-'. Get one at https://openrouter.ai/keys".into()
                        ));
                    }

                    // Warn but allow if doesn't match expected format
                    if !input.starts_with("sk-or-") {
                        return Ok(inquire::validator::Validation::Invalid(
                            "OpenRouter API keys typically start with 'sk-or-'. Please verify your key or get one at https://openrouter.ai/keys".into()
                        ));
                    }

                    Ok(inquire::validator::Validation::Valid)
                })
                .prompt();

            match api_key {
                Ok(key) => {
                    let key = key.trim().to_string();
                    config.openrouter_key = Some(key);
                    save_config(config)?;
                    println!();
                    println!("{GREEN}[✓]{RESET} OpenRouter API key saved successfully!");
                    println!("{DIM}    Cloud fallback is now enabled for complex queries.{RESET}");
                    println!();
                }
                Err(_) => {
                    // User pressed Esc or cancelled
                    println!();
                    println!("{YELLOW}[!]{RESET} Setup cancelled. You can set this up later with:");
                    println!("    {CYAN}rigrun config --openrouter-key YOUR_KEY{RESET}");
                    println!();
                }
            }
        }
        "No, local-only for now" => {
            println!();
            println!("{GREEN}[✓]{RESET} No problem! You can set this up later with {CYAN}`rigrun config`{RESET}");
            println!();
        }
        _ => {}
    }

    Ok(())
}

/// Opens a URL in the default browser
fn open_browser(url: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("cmd")
            .args(["/C", "start", "", url])
            .spawn()?;
        Ok(())
    }

    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg(url)
            .spawn()?;
        return Ok(());
    }

    #[cfg(target_os = "linux")]
    {
        std::process::Command::new("xdg-open")
            .arg(url)
            .spawn()?;
        return Ok(());
    }

    #[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
    {
        anyhow::bail!("Browser opening not supported on this platform");
    }
}

fn handle_ide_setup() -> Result<()> {
    println!();
    println!("{CYAN}{BOLD}=== IDE Integration Setup ==={RESET}");
    println!();
    println!("{WHITE}RigRun runs an OpenAI-compatible API server at:{RESET}");
    println!("  {CYAN}{BOLD}http://localhost:8787{RESET}");
    println!();
    println!("{WHITE}You can configure your IDE to use rigrun as the backend:{RESET}");
    println!();

    // VS Code / Cursor
    println!("{CYAN}{BOLD}VS Code / Cursor (via Continue extension):{RESET}");
    println!("  1. Install the Continue extension");
    println!("  2. Open Continue settings (gear icon)");
    println!("  3. Add a custom model:");
    println!();
    println!("     {DIM}{{");
    println!("       \"models\": [{{");
    println!("         \"title\": \"RigRun Local\",");
    println!("         \"provider\": \"openai\",");
    println!("         \"model\": \"auto\",");
    println!("         \"apiBase\": \"http://localhost:8787/v1\",");
    println!("         \"apiKey\": \"not-needed\"");
    println!("       }}]");
    println!("     }}{RESET}");
    println!();

    // JetBrains
    println!("{CYAN}{BOLD}JetBrains IDEs (IntelliJ, PyCharm, etc):{RESET}");
    println!("  1. Install AI Assistant or compatible plugin");
    println!("  2. Go to Settings → AI Assistant");
    println!("  3. Add Custom LLM Provider:");
    println!("     - URL: {CYAN}http://localhost:8787/v1/chat/completions{RESET}");
    println!("     - API Key: {DIM}(leave empty or use any value){RESET}");
    println!();

    // Neovim
    println!("{CYAN}{BOLD}Neovim (via copilot.lua or similar):{RESET}");
    println!("  Configure your AI plugin to use:");
    println!("  - Endpoint: {CYAN}http://localhost:8787/v1/chat/completions{RESET}");
    println!("  - Model: {WHITE}auto{RESET}");
    println!();

    // General curl example
    println!("{CYAN}{BOLD}Or test it directly with curl:{RESET}");
    println!("  {WHITE}{BOLD}curl http://localhost:8787/v1/chat/completions \\{RESET}");
    println!("    {WHITE}{BOLD}-H \"Content-Type: application/json\" \\{RESET}");
    println!("    {WHITE}{BOLD}-d '{{\"model\":\"auto\",\"messages\":[{{\"role\":\"user\",\"content\":\"hi\"}}]}}'{RESET}");
    println!();

    println!("{GREEN}[✓]{RESET} Hope that helps!");
    println!();
    println!("{DIM}Got it! Returning to menu...{RESET}");
    std::thread::sleep(std::time::Duration::from_millis(1200));

    Ok(())
}

fn handle_background_server() -> Result<()> {
    println!();
    println!("{CYAN}{BOLD}=== Background Server Setup ==={RESET}");
    println!();

    #[cfg(target_os = "windows")]
    {
        use inquire::Select;

        let options = vec![
            "Start background server with live stats dashboard",
            "Install as Windows startup task",
            "Show manual setup instructions",
            "Cancel",
        ];

        let answer = Select::new("Choose an option:", options)
            .with_help_message("↑↓ to move, enter to select")
            .prompt();

        match answer {
            Ok("Start background server with live stats dashboard") => {
                println!();
                println!("{GREEN}[✓]{RESET} Starting rigrun as background server...");

                // Get the current executable path
                let exe_path = std::env::current_exe()?;
                let exe_str = exe_path.display().to_string();

                // Start rigrun in a new visible terminal window showing the stats dashboard
                // Note: "start" requires the title to be quoted, and the command path is passed directly
                let status = std::process::Command::new("cmd")
                    .args([
                        "/C",
                        "start",
                        "\"RigRun Server\"",  // Window title (must be quoted for start command)
                        &exe_str,              // Direct path - no extra quotes needed
                    ])
                    .spawn();

                match status {
                    Ok(_) => {
                        // Wait a moment for the window to open
                        std::thread::sleep(std::time::Duration::from_millis(1500));

                        println!("{GREEN}[✓]{RESET} Background server started in new window!");
                        println!();
                        println!("{WHITE}Access at: {CYAN}{BOLD}http://localhost:8787{RESET}");
                        println!();
                        println!("{DIM}The server is running in a separate window with live stats.{RESET}");
                        println!("{DIM}Close that window to stop the server, or use: {WHITE}rigrun stop{RESET}");
                    }
                    Err(e) => {
                        println!("{RED}[✗]{RESET} Failed to start background server: {}", e);
                    }
                }

                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            Ok("Install as Windows startup task") => {
                println!();
                install_windows_startup_task()?;

                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            Ok("Show manual setup instructions") => {
                println!();
                show_manual_background_setup();

                println!();
                println!("{DIM}Press Enter to return to menu...{RESET}");
                let stdin = std::io::stdin();
                let mut input = String::new();
                stdin.read_line(&mut input)?;
            }
            Ok("Cancel") => {
                println!();
                println!("{YELLOW}[!]{RESET} Cancelled.");
                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            _ => {}
        }
    }

    #[cfg(target_os = "linux")]
    {
        use inquire::Select;

        let options = vec![
            "Start background server now",
            "Show systemd setup instructions",
            "Cancel",
        ];

        let answer = Select::new("Choose an option:", options)
            .with_help_message("↑↓ to move, enter to select")
            .prompt();

        match answer {
            Ok("Start background server now") => {
                println!();
                println!("{GREEN}[✓]{RESET} Starting rigrun as background server...");

                let exe_path = std::env::current_exe()?;

                // Start in background with nohup
                let status = std::process::Command::new("nohup")
                    .arg(&exe_path)
                    .stdin(std::process::Stdio::null())
                    .stdout(std::process::Stdio::null())
                    .stderr(std::process::Stdio::null())
                    .spawn();

                match status {
                    Ok(child) => {
                        let pid = child.id();
                        println!("{GREEN}[✓]{RESET} Background server started (PID: {pid})!");
                        println!();
                        println!("{WHITE}Access at: {CYAN}{BOLD}http://localhost:8787{RESET}");
                        println!();
                        println!("{DIM}Stop with: {WHITE}rigrun stop{RESET}");
                    }
                    Err(e) => {
                        println!("{RED}[✗]{RESET} Failed to start background server: {}", e);
                    }
                }

                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            Ok("Show systemd setup instructions") => {
                println!();
                show_manual_background_setup();

                println!();
                println!("{DIM}Press Enter to return to menu...{RESET}");
                let stdin = std::io::stdin();
                let mut input = String::new();
                stdin.read_line(&mut input)?;
            }
            Ok("Cancel") => {
                println!();
                println!("{YELLOW}[!]{RESET} Cancelled.");
                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            _ => {}
        }
    }

    #[cfg(target_os = "macos")]
    {
        use inquire::Select;

        let options = vec![
            "Start background server now",
            "Show launchd setup instructions",
            "Cancel",
        ];

        let answer = Select::new("Choose an option:", options)
            .with_help_message("↑↓ to move, enter to select")
            .prompt();

        match answer {
            Ok("Start background server now") => {
                println!();
                println!("{GREEN}[✓]{RESET} Starting rigrun as background server...");

                let exe_path = std::env::current_exe()?;

                // Start in background
                let status = std::process::Command::new("nohup")
                    .arg(&exe_path)
                    .stdin(std::process::Stdio::null())
                    .stdout(std::process::Stdio::null())
                    .stderr(std::process::Stdio::null())
                    .spawn();

                match status {
                    Ok(child) => {
                        let pid = child.id();
                        println!("{GREEN}[✓]{RESET} Background server started (PID: {pid})!");
                        println!();
                        println!("{WHITE}Access at: {CYAN}{BOLD}http://localhost:8787{RESET}");
                        println!();
                        println!("{DIM}Stop with: {WHITE}rigrun stop{RESET}");
                    }
                    Err(e) => {
                        println!("{RED}[✗]{RESET} Failed to start background server: {}", e);
                    }
                }

                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            Ok("Show launchd setup instructions") => {
                println!();
                show_manual_background_setup();

                println!();
                println!("{DIM}Press Enter to return to menu...{RESET}");
                let stdin = std::io::stdin();
                let mut input = String::new();
                stdin.read_line(&mut input)?;
            }
            Ok("Cancel") => {
                println!();
                println!("{YELLOW}[!]{RESET} Cancelled.");
                println!();
                println!("{DIM}Got it! Returning to menu...{RESET}");
                std::thread::sleep(std::time::Duration::from_millis(1200));
            }
            _ => {}
        }
    }

    Ok(())
}

#[cfg(target_os = "windows")]
fn install_windows_startup_task() -> Result<()> {
    println!("{CYAN}[...]{RESET} Installing Windows startup task...");
    println!();

    // Get the current executable path
    let exe_path = std::env::current_exe()?;
    let exe_path_str = exe_path.display().to_string();

    // Create a scheduled task using schtasks.exe
    // This will run rigrun on user login, minimized
    let task_name = "RigRun-Server";

    // First, check if task already exists and delete it
    let _ = std::process::Command::new("schtasks")
        .args(["/Delete", "/TN", task_name, "/F"])
        .output();

    // Create the new task
    let status = std::process::Command::new("schtasks")
        .args([
            "/Create",
            "/TN", task_name,
            "/TR", &exe_path_str,
            "/SC", "ONLOGON",
            "/RL", "LIMITED",
            "/F",
        ])
        .status();

    match status {
        Ok(s) if s.success() => {
            println!("{GREEN}[✓]{RESET} Startup task installed successfully!");
            println!();
            println!("{WHITE}Task name:{RESET} {CYAN}{task_name}{RESET}");
            println!("{WHITE}Trigger:{RESET} On user login");
            println!("{WHITE}Command:{RESET} {DIM}{}{RESET}", exe_path_str);
            println!();
            println!("{GREEN}[✓]{RESET} RigRun will now start automatically when you log in!");
            println!();
            println!("{DIM}To remove this task later:{RESET}");
            println!("  {WHITE}schtasks /Delete /TN \"{task_name}\" /F{RESET}");
            println!();
            println!("{DIM}Or use:{RESET} {WHITE}Task Scheduler{RESET} {DIM}(search in Start menu){RESET}");
        }
        Ok(_) => {
            println!("{RED}[✗]{RESET} Failed to create startup task");
            println!();
            println!("{YELLOW}[!]{RESET} You may need administrator privileges.");
            println!();
            println!("{DIM}Try running this command manually as administrator:{RESET}");
            println!("  {WHITE}schtasks /Create /TN \"{task_name}\" /TR \"{}\" /SC ONLOGON /RL LIMITED /F{RESET}", exe_path_str);
        }
        Err(e) => {
            println!("{RED}[✗]{RESET} Error creating startup task: {}", e);
        }
    }

    Ok(())
}

fn show_manual_background_setup() {
    #[cfg(target_os = "windows")]
    {
        println!("{CYAN}{BOLD}Windows Manual Setup:{RESET}");
        println!();
        println!("{WHITE}Option 1: Task Scheduler (Recommended){RESET}");
        println!("  1. Open Task Scheduler (search in Start menu)");
        println!("  2. Click 'Create Basic Task'");
        println!("  3. Name: {CYAN}RigRun Server{RESET}");
        println!("  4. Trigger: {CYAN}When I log on{RESET}");
        println!("  5. Action: {CYAN}Start a program{RESET}");

        if let Ok(exe_path) = std::env::current_exe() {
            println!("  6. Program: {WHITE}{}{RESET}", exe_path.display());
        } else {
            println!("  6. Program: {WHITE}<path-to-rigrun.exe>{RESET}");
        }
        println!();

        println!("{WHITE}Option 2: Startup Folder{RESET}");
        println!("  1. Press {WHITE}Win+R{RESET}");
        println!("  2. Type: {CYAN}shell:startup{RESET}");
        println!("  3. Create a shortcut to rigrun.exe in that folder");
        println!();

        println!("{WHITE}Option 3: NSSM (Advanced){RESET}");
        println!("  1. Download NSSM from {CYAN}https://nssm.cc{RESET}");
        if let Ok(exe_path) = std::env::current_exe() {
            println!("  2. Run: {CYAN}nssm install RigRun \"{}\"{RESET}", exe_path.display());
        } else {
            println!("  2. Run: {CYAN}nssm install RigRun \"<path-to-rigrun.exe>\"{RESET}");
        }
        println!("  3. Configure service settings as desired");
        println!("  4. Start the service");
    }

    #[cfg(target_os = "linux")]
    {
        println!("{CYAN}{BOLD}Linux (systemd) Manual Setup:{RESET}");
        println!();
        println!("1. Create a systemd service file:");
        println!("   {WHITE}/etc/systemd/system/rigrun.service{RESET}");
        println!();
        println!("2. Add this content:");
        println!("   {DIM}[Unit]");
        println!("   Description=RigRun Local LLM Router");
        println!("   After=network.target");
        println!();
        println!("   [Service]");
        println!("   Type=simple");
        println!("   User=$USER");

        if let Ok(exe_path) = std::env::current_exe() {
            println!("   ExecStart={}", exe_path.display());
        } else {
            println!("   ExecStart=/usr/local/bin/rigrun");
        }

        println!("   Restart=always");
        println!();
        println!("   [Install]");
        println!("   WantedBy=multi-user.target{RESET}");
        println!();
        println!("3. Enable and start:");
        println!("   {WHITE}sudo systemctl enable rigrun{RESET}");
        println!("   {WHITE}sudo systemctl start rigrun{RESET}");
    }

    #[cfg(target_os = "macos")]
    {
        println!("{CYAN}{BOLD}macOS (launchd) Manual Setup:{RESET}");
        println!();
        println!("1. Create a plist file:");
        println!("   {WHITE}~/Library/LaunchAgents/com.rigrun.server.plist{RESET}");
        println!();
        println!("2. Add this content:");
        println!("   {DIM}<?xml version=\"1.0\" encoding=\"UTF-8\"?>");
        println!("   <!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\"");
        println!("     \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">");
        println!("   <plist version=\"1.0\">");
        println!("   <dict>");
        println!("     <key>Label</key>");
        println!("     <string>com.rigrun.server</string>");
        println!("     <key>ProgramArguments</key>");
        println!("     <array>");

        if let Ok(exe_path) = std::env::current_exe() {
            println!("       <string>{}</string>", exe_path.display());
        } else {
            println!("       <string>/usr/local/bin/rigrun</string>");
        }

        println!("     </array>");
        println!("     <key>RunAtLoad</key>");
        println!("     <true/>");
        println!("     <key>KeepAlive</key>");
        println!("     <true/>");
        println!("   </dict>");
        println!("   </plist>{RESET}");
        println!();
        println!("3. Load the service:");
        println!("   {WHITE}launchctl load ~/Library/LaunchAgents/com.rigrun.server.plist{RESET}");
    }
}

fn handle_learn_more() -> Result<()> {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== Learn More About These Options ==={RESET}");
    println!();

    // Option 1: Start chatting now
    println!("{CYAN}{BOLD}💬 Start chatting now{RESET}");
    println!("{DIM}   Opens an interactive chat session right in your terminal{RESET}");
    println!("{DIM}   Great for quick questions, code generation, debugging help{RESET}");
    println!("{DIM}   Conversation context is maintained throughout the session{RESET}");
    println!("{DIM}   Type 'exit' or Ctrl+C to end{RESET}");
    println!();

    // Option 2: Set up my IDE
    println!("{CYAN}{BOLD}🔧 Set up my IDE{RESET}");
    println!("{DIM}   Configures your code editor to use rigrun as an AI backend{RESET}");
    println!("{DIM}   Supports VS Code, Cursor, JetBrains, Neovim, and more{RESET}");
    println!("{DIM}   Your queries go through rigrun's smart routing:{RESET}");
    println!("{DIM}     • Cache hit → instant response ($0){RESET}");
    println!("{DIM}     • Local GPU → your hardware ($0){RESET}");
    println!("{DIM}     • Cloud → only when needed (pay per use){RESET}");
    println!();

    // Option 3: CLI examples
    println!("{CYAN}{BOLD}📋 CLI examples{RESET}");
    println!("{DIM}   Shows common command patterns:{RESET}");
    println!("{DIM}   • Direct prompts: {WHITE}rigrun \"explain this code\"{DIM}{RESET}");
    println!("{DIM}   • Piping: {WHITE}cat file.rs | rigrun \"review this\"{DIM}{RESET}");
    println!("{DIM}   • Interactive: {WHITE}rigrun chat{DIM}{RESET}");
    println!();

    // Option 4: Background server
    println!("{CYAN}{BOLD}🚀 Background server{RESET}");
    println!("{DIM}   Runs rigrun as a system service/daemon{RESET}");
    println!("{DIM}   • Starts automatically on boot{RESET}");
    println!("{DIM}   • Always available for IDE integration{RESET}");
    println!("{DIM}   • Low memory footprint when idle{RESET}");
    println!("{DIM}   • Access via {WHITE}http://localhost:8787{DIM}{RESET}");
    println!();

    // Auto-return to menu after a brief pause
    println!("{DIM}Got it! Returning to menu...{RESET}");
    std::thread::sleep(std::time::Duration::from_millis(1200));

    Ok(())
}

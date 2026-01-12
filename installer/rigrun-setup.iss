; rigrun Windows Installer - Elite Edition
; Built with Inno Setup - https://jrsoftware.org/isinfo.php

#define MyAppName "rigrun"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "rigrun"
#define MyAppURL "https://rigrun.dev"
#define MyAppExeName "rigrun.exe"

[Setup]
; App identity
AppId={{8F3B5A2E-7C4D-4E9F-B8A1-2D6F9E3C5B7A}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}/releases

; Install location
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes

; Output
OutputDir=output
OutputBaseFilename=rigrun-{#MyAppVersion}-setup
; SetupIconFile=..\assets\icon.ico  ; Uncomment when icon is added
UninstallDisplayIcon={app}\{#MyAppExeName}

; Compression
Compression=lzma2/ultra64
SolidCompression=yes

; Privileges (user-level install by default)
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

; Modern, professional look
WizardStyle=modern
WizardSizePercent=120,100
DisableWelcomePage=no
SetupLogging=yes

; License
LicenseFile=..\LICENSE

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "addtopath"; Description: "Add rigrun to PATH (recommended)"; GroupDescription: "Additional options:"; Flags: checkedonce
Name: "desktopicon"; Description: "Create desktop shortcut"; GroupDescription: "Additional options:"; Flags: unchecked
Name: "installollama"; Description: "Install Ollama (required for local inference)"; GroupDescription: "Dependencies:"; Flags: checkedonce; Check: not IsOllamaInstalled

[Files]
; Main executable
Source: "..\target\release\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

; Readme
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Install Ollama if checked
Filename: "{tmp}\OllamaSetup.exe"; Description: "Installing Ollama..."; StatusMsg: "Installing Ollama..."; Tasks: installollama; Flags: waituntilterminated skipifsilent

; Start rigrun server in a new terminal window after install
Filename: "{cmd}"; Parameters: "/k ""{app}\rigrun.exe"""; Description: "Start rigrun server"; Flags: nowait postinstall skipifsilent

[Code]
var
  WelcomePage: TWizardPage;
  WelcomeMemo: TNewMemo;
  FinishPage: TWizardPage;
  FinishMemo: TNewMemo;

// Check if Ollama is already installed
function IsOllamaInstalled: Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec('cmd.exe', '/c ollama --version', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

// Create custom welcome page with ASCII art
procedure CreateWelcomePage;
var
  AsciiArt: string;
begin
  WelcomePage := CreateCustomPage(wpLicense, 'Welcome to rigrun', 'Your GPU first. Cloud when needed.');

  AsciiArt :=
    '   ____  _       ____' + #13#10 +
    '  |  _ \(_) __ _|  _ \ _   _ _ __' + #13#10 +
    '  | |_) | |/ _` | |_) | | | | ''_ \' + #13#10 +
    '  |  _ <| | (_| |  _ <| |_| | | | |' + #13#10 +
    '  |_| \_\_|\__, |_| \_\\__,_|_| |_|' + #13#10 +
    '           |___/  v{#MyAppVersion}' + #13#10 +
    '' + #13#10 +
    'A local-first LLM router that puts your hardware to work and' + #13#10 +
    'saves you money.' + #13#10 +
    '' + #13#10 +
    '                 Query' + #13#10 +
    '                   |' + #13#10 +
    '                   v' + #13#10 +
    '    +-----------------------------+' + #13#10 +
    '    |         rigrun              |' + #13#10 +
    '    +-----------------------------+' + #13#10 +
    '                   |' + #13#10 +
    '    +--------------+--------------+' + #13#10 +
    '    |              |              |' + #13#10 +
    '    v              v              v' + #13#10 +
    ' [CACHE]        [LOCAL]        [CLOUD]' + #13#10 +
    ' (instant)    (your GPU)    (when needed)' + #13#10 +
    '    $0            $0         Haiku/Sonnet' + #13#10 +
    '' + #13#10 +
    'THE PROBLEM:' + #13#10 +
    '  • $100-500/month on LLM APIs for the average dev' + #13#10 +
    '  • 85% of queries don''t need expensive models' + #13#10 +
    '  • Your $2,000 GPU sits idle while you pay for cloud' + #13#10 +
    '' + #13#10 +
    'THE SOLUTION:' + #13#10 +
    '  rigrun routes queries through a tiered system:' + #13#10 +
    '  1. Cache - Instant response for repeated queries ($0)' + #13#10 +
    '  2. Local - Your GPU via Ollama ($0, private, fast)' + #13#10 +
    '  3. Cloud - Haiku/Sonnet/Opus only when needed' + #13#10 +
    '' + #13#10 +
    'RESULT: 80-90% cost savings' + #13#10 +
    '' + #13#10 +
    'Click Next to continue with the installation.';

  WelcomeMemo := TNewMemo.Create(WelcomePage);
  WelcomeMemo.Parent := WelcomePage.Surface;
  WelcomeMemo.Left := 0;
  WelcomeMemo.Top := 0;
  WelcomeMemo.Width := WelcomePage.SurfaceWidth;
  WelcomeMemo.Height := WelcomePage.SurfaceHeight;
  WelcomeMemo.ScrollBars := ssVertical;
  WelcomeMemo.ReadOnly := True;
  WelcomeMemo.Font.Name := 'Consolas';
  WelcomeMemo.Font.Size := 9;
  WelcomeMemo.Text := AsciiArt;
  WelcomeMemo.Color := clWindow;
end;

// Create custom finish page with quick start info
procedure CreateFinishPage;
begin
  FinishPage := CreateCustomPage(wpInstalling, 'Installation Complete!', 'rigrun is ready to use');

  FinishMemo := TNewMemo.Create(FinishPage);
  FinishMemo.Parent := FinishPage.Surface;
  FinishMemo.Left := 0;
  FinishMemo.Top := 0;
  FinishMemo.Width := FinishPage.SurfaceWidth;
  FinishMemo.Height := FinishPage.SurfaceHeight;
  FinishMemo.ScrollBars := ssVertical;
  FinishMemo.ReadOnly := True;
  FinishMemo.Font.Name := 'Consolas';
  FinishMemo.Font.Size := 9;
  FinishMemo.Color := clWindow;
  FinishMemo.Text := 'Installation in progress...';
end;

// Update finish page text with actual install path
procedure UpdateFinishPage;
var
  QuickStartText: string;
begin
  QuickStartText :=
    '===========================================================' + #13#10 +
    '  INSTALLATION SUCCESSFUL!' + #13#10 +
    '===========================================================' + #13#10 +
    '' + #13#10 +
    'rigrun has been installed to:' + #13#10 +
    '  ' + ExpandConstant('{app}') + #13#10 +
    '' + #13#10 +
    'QUICK START:' + #13#10 +
    '' + #13#10 +
    '  1. Open a new terminal (Command Prompt or PowerShell)' + #13#10 +
    '' + #13#10 +
    '  2. Run rigrun:' + #13#10 +
    '     > rigrun' + #13#10 +
    '' + #13#10 +
    '     First run will:' + #13#10 +
    '     • Detect your GPU and VRAM' + #13#10 +
    '     • Select the best model for your hardware' + #13#10 +
    '     • Download the model via Ollama' + #13#10 +
    '     • Start an OpenAI-compatible server' + #13#10 +
    '' + #13#10 +
    'USEFUL COMMANDS:' + #13#10 +
    '' + #13#10 +
    '  rigrun              Start the server (auto-detects GPU)' + #13#10 +
    '  rigrun status       Show current stats and GPU info' + #13#10 +
    '  rigrun models       List available models' + #13#10 +
    '  rigrun pull <name>  Download a specific model' + #13#10 +
    '  rigrun config       Configure settings' + #13#10 +
    '' + #13#10 +
    'DOCUMENTATION:' + #13#10 +
    '' + #13#10 +
    '  Website: https://rigrun.dev' + #13#10 +
    '  GitHub:  https://github.com/rigrun/rigrun' + #13#10 +
    '' + #13#10 +
    'EXAMPLE USAGE:' + #13#10 +
    '' + #13#10 +
    '  curl localhost:8787/v1/chat/completions \' + #13#10 +
    '    -H "Content-Type: application/json" \' + #13#10 +
    '    -d ''{"model":"auto","messages":[{' + #13#10 +
    '        "role":"user","content":"hi"}]}''' + #13#10 +
    '' + #13#10 +
    '===========================================================' + #13#10 +
    '  Put your rig to work. Save 90%.' + #13#10 +
    '===========================================================';

  FinishMemo.Text := QuickStartText;
end;

// Initialize custom wizard pages
procedure InitializeWizard;
begin
  CreateWelcomePage;
  CreateFinishPage;
end;

// Ensure the finish page is shown
function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  // Never skip our custom finish page
  if PageID = FinishPage.ID then
    Result := False;
end;

// Show finish page before the final "finished" dialog
procedure CurPageChanged(CurPageID: Integer);
begin
  // Update our custom finish page when we reach it
  if CurPageID = FinishPage.ID then
  begin
    UpdateFinishPage;
  end;
  // Keep the installer window open on the final page
  if CurPageID = wpFinished then
  begin
    WizardForm.NextButton.Caption := '&Finish';
  end;
end;

// Windows API constants for broadcasting environment changes
const
  WM_SETTINGCHANGE = $001A;
  SMTO_ABORTIFHUNG = $0002;

// Declare Windows API function to broadcast environment change
function SendMessageTimeout(hWnd: HWND; Msg: UINT; wParam: Longint; lParam: PAnsiChar;
  fuFlags: UINT; uTimeout: UINT; var lpdwResult: DWORD): Longint;
  external 'SendMessageTimeoutA@user32.dll stdcall';

// Add to PATH after install
procedure AddToPath();
var
  Path: string;
  AppDir: string;
  ResultCode: DWORD;
begin
  if WizardIsTaskSelected('addtopath') then
  begin
    AppDir := ExpandConstant('{app}');

    // Read current PATH from registry
    if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
      Path := '';

    // Check if AppDir is already in PATH
    if Pos(Uppercase(AppDir), Uppercase(Path)) = 0 then
    begin
      // Add to PATH
      if Path <> '' then
      begin
        // Only add semicolon if PATH doesn't already end with one
        if Copy(Path, Length(Path), 1) <> ';' then
          Path := Path + ';';
      end;
      Path := Path + AppDir;

      // Write to registry
      if RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
      begin
        Log('Successfully added to PATH: ' + AppDir);

        // Broadcast environment change to all windows
        SendMessageTimeout(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
          'Environment', SMTO_ABORTIFHUNG, 5000, ResultCode);
        Log('Broadcast environment change notification');
      end
      else
      begin
        Log('ERROR: Failed to write PATH to registry');
        MsgBox('Warning: Failed to add rigrun to PATH. You may need to add it manually.', mbError, MB_OK);
      end;
    end
    else
    begin
      Log('Already in PATH: ' + AppDir);
    end;
  end;
end;

// Download Ollama installer if needed, and add to PATH
procedure CurStepChanged(CurStep: TSetupStep);
var
  DownloadPage: TDownloadWizardPage;
begin
  if CurStep = ssInstall then
  begin
    if WizardIsTaskSelected('installollama') then
    begin
      DownloadPage := CreateDownloadPage(
        'Downloading Ollama',
        'Downloading Ollama installer (required for local inference)...',
        nil
      );
      DownloadPage.Clear;
      DownloadPage.Add('https://ollama.com/download/OllamaSetup.exe', 'OllamaSetup.exe', '');
      DownloadPage.Show;
      try
        DownloadPage.Download;
        Log('Successfully downloaded Ollama installer');
      except
        // Download failed, warn but continue
        MsgBox('Failed to download Ollama installer. You can download it manually from https://ollama.com', mbError, MB_OK);
        Log('Failed to download Ollama installer');
      end;
      DownloadPage.Hide;
    end;
  end
  else if CurStep = ssPostInstall then
  begin
    AddToPath();
  end;
end;

// Remove from PATH on uninstall
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
  AppDir: string;
  P: Integer;
  ResultCode: DWORD;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    AppDir := ExpandConstant('{app}');
    if RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
    begin
      P := Pos(Uppercase(AppDir), Uppercase(Path));
      if P <> 0 then
      begin
        // Remove the directory from PATH
        Delete(Path, P, Length(AppDir));

        // Clean up extra semicolons
        StringChangeEx(Path, ';;', ';', True);
        if (Length(Path) > 0) and (Path[1] = ';') then
          Delete(Path, 1, 1);
        if (Length(Path) > 0) and (Path[Length(Path)] = ';') then
          Delete(Path, Length(Path), 1);

        // Write back to registry
        if RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
        begin
          Log('Successfully removed from PATH: ' + AppDir);

          // Broadcast environment change to all windows
          SendMessageTimeout(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
            'Environment', SMTO_ABORTIFHUNG, 5000, ResultCode);
          Log('Broadcast environment change notification');
        end
        else
        begin
          Log('ERROR: Failed to write PATH to registry during uninstall');
        end;
      end;
    end;
  end;
end;

[Messages]
WelcomeLabel1=Welcome to rigrun
WelcomeLabel2=Your GPU first. Cloud when needed.%n%nThis will install rigrun {#MyAppVersion} on your computer.
FinishedHeadingLabel=Installation Complete!
FinishedLabelNoIcons=rigrun has been successfully installed on your system.
FinishedLabel=rigrun has been successfully installed on your system.

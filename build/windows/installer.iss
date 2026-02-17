; VPN Client - Inno Setup Installer Script
; Requirements: Inno Setup 6+ (https://jrsoftware.org/isinfo.php)
;
; Build:  iscc installer.iss
; Or via Makefile: make installer-windows

#define MyAppName "VPN Client"
#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif
#define MyAppPublisher "VPN Client"
#define MyAppURL "https://github.com/user/vpn-client"
#define MyAppExeName "vpn-client.exe"

[Setup]
AppId={{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
; Require admin for installation (VPN needs admin rights)
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog
OutputDir=..\..\dist
OutputBaseFilename=vpn-client-{#MyAppVersion}-windows-setup
SetupIconFile=..\..\build\windows\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
; Minimum Windows 10
MinVersion=10.0
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesInstallIn64BitMode=x64compatible
ArchitecturesAllowed=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "startupentry"; Description: "Start VPN Client on Windows startup"; GroupDescription: "Startup:"

[Files]
; Main executable
Source: "..\..\dist\windows-amd64\vpn-client.exe"; DestDir: "{app}"; Flags: ignoreversion
; Example config (next to exe, where the app expects it)
Source: "..\..\configs\config.example.yaml"; DestDir: "{app}"; DestName: "config.yaml"; Flags: onlyifdoesntexist uninsneveruninstall
; License (if exists)
; Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion; DestName: "LICENSE.txt"

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
; Startup entry
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#MyAppName}"; ValueData: """{app}\{#MyAppExeName}"""; Flags: uninsdeletevalue; Tasks: startupentry

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent runascurrentuser

[UninstallRun]
; Ensure VPN is disconnected before uninstall
Filename: "{cmd}"; Parameters: "/C taskkill /F /IM vpn-client.exe"; Flags: runhidden; RunOnceId: "KillVPNClient"

[UninstallDelete]
Type: filesandordirs; Name: "{app}\configs"
Type: filesandordirs; Name: "{app}\logs"

[Code]
// Kill running instance before installation
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  ResultCode: Integer;
begin
  Exec('taskkill', '/F /IM vpn-client.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := '';
end;

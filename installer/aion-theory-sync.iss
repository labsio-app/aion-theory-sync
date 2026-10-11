; Inno Setup script for the Aion Theory Sync installer.
; Built in CI (.github/workflows/release.yml) after the Go build, with:
;   iscc /DMyAppVersion=1.2.3 installer\aion-theory-sync.iss
; MyAppVersion defaults to 0.0.0-dev for a local, manual build.
#ifndef MyAppVersion
#define MyAppVersion "0.0.0-dev"
#endif

#define MyAppName "Aion Theory Sync"
#define MyAppPublisher "labsio-app"
#define MyAppURL "https://github.com/labsio-app/aion-theory-sync"
#define MyAppExeName "aion-theory-sync-gui.exe"

[Setup]
AppId={{B6C9E4A0-6B8E-4B4A-9B2F-6A2E2D9B6C1A}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}/releases
; Installing to Program Files needs an admin prompt once; a per-user
; install (no UAC prompt) is offered as the default lower-friction choice,
; matching a small community tool more than an admin-run with no publisher
; signature can't elevate.
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
UsePreviousAppDir=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
DisableProgramGroupPage=yes
LicenseFile=..\LICENSE
OutputDir=.
OutputBaseFilename=aion-theory-sync-setup
SetupIconFile=..\assets\icon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
; Both binaries, built straight with `go build` into build\ right before
; iscc runs (see release.yml) — not goreleaser's dist/, whose per-build
; directory naming this script would otherwise have to guess at.
Source: "..\build\aion-theory-sync-gui.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\aion-theory-sync.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\assets\icon.ico"; DestDir: "{app}\assets"; Flags: ignoreversion
Source: "..\assets\logo.png"; DestDir: "{app}\assets"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\assets\icon.ico"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\assets\icon.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

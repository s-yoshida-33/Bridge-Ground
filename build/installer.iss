[Setup]
AppId={{6E8A1F3C-2B4D-4A7E-9C5F-D8E1234B567A}
AppName=Bridge Ground
AppVersion=4.1.1
AppPublisher=Toei Techno International Inc.
DefaultDirName={localappdata}\Bridge Ground
PrivilegesRequired=lowest
OutputDir=..\release
OutputBaseFilename=BridgeGroundSetup-x64-4.1.1
Compression=lzma
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
WizardStyle=modern
UninstallDisplayName=Bridge Ground
CloseApplications=yes

[Tasks]
Name: "desktopicon"; Description: "デスクトップにショートカットを作成する"; GroupDescription: "追加タスク:"

[Files]
Source: "dist\\bridge-ground.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\\src\\*"; DestDir: "{app}\\src"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\\Bridge Ground"; Filename: "{app}\\bridge-ground.exe"
Name: "{autodesktop}\\Bridge Ground"; Filename: "{app}\\bridge-ground.exe"; Tasks: desktopicon

[UninstallRun]
Filename: "{cmd}"; Parameters: "/c schtasks /delete /tn ""Bridge Ground Auto Start"" /f"; Flags: runhidden waituntilterminated

[Run]
Filename: "{app}\\bridge-ground.exe"; Description: "Bridge Groundを起動する"; Flags: nowait postinstall skipifsilent

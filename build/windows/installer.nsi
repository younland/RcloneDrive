; RcloneDrive 安装器（管理员级）。
;
; mygo 生成的 NSIS 脚本没有定制注入点，且是按用户级安装（%LOCALAPPDATA%，
; 不提权），无法在 setup 阶段安装 WinFsp。这里按 mygo 安装器的结构重写为
; 管理员级：setup 启动时一次 UAC，随后应用文件 + WinFsp（如未安装）一并
; 静默装完。输出文件名与 mygo 约定一致（更新器按此命名匹配资产）。
;
; 编译：makensis /DVERSION=x.y.z installer.nsi（由 Taskfile 的 package 任务
; 在 go tool mygo build 之后调用，输出覆盖 mygo 自带的安装包）。
; 路径约定：相对路径以本脚本所在目录（build/windows）为基准。

Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

!ifndef VERSION
  !error "用法: makensis /DVERSION=x.y.z installer.nsi"
!endif
!define ROOT "..\.."

!define STAGE "${ROOT}\build\windows-amd64"
!define WINFSP_MSI "${ROOT}\third_party\winfsp\winfsp-2.1.25156.msi"

Name "RcloneDrive"
OutFile "${STAGE}\RcloneDrive Setup ${VERSION} amd64.exe"
InstallDir "$PROGRAMFILES64\RcloneDrive"
RequestExecutionLevel admin
BrandingText " "

!define MUI_ICON "${ROOT}\build\windows\appicon.ico"
!define MUI_UNICON "${ROOT}\build\windows\appicon.ico"
; 完成页"运行"走 LaunchUnelevated：安装器是管理员级，直接 Exec 会让应用
; 继承提权 token，而提权进程挂载的盘符对桌面（非提权）会话不可见。
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_FUNCTION "LaunchUnelevated"
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "创建桌面快捷方式"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!include "MUI2.nsh"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  SetRegView 64
FunctionEnd

Section "RcloneDrive" SEC_APP
  ; 覆盖安装/升级前结束运行中的实例，否则 exe 文件锁导致写入失败
  ; （Abort/Retry/Ignore 弹窗）。/F 必须带：应用关闭窗口是隐藏到托盘，
  ; 不带 /F 的 taskkill 只发 WM_CLOSE，会被 OnClose 拦截成隐藏。
  ; 强杀跳过挂载清理：WinFsp 卷随进程死亡自动卸载，重启应用后自动重连。
  ExecWait 'taskkill /IM RcloneDrive.exe /F'
  Sleep 500 ; 等系统释放文件句柄
  SetOutPath "$INSTDIR"
  ; 暂存目录里可能留有本安装器自身的旧产物（RcloneDrive Setup*.exe），排除
  File /r /x "RcloneDrive Setup*.exe" "${STAGE}\*.*"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortCut "$SMPROGRAMS\RcloneDrive.lnk" "$INSTDIR\RcloneDrive.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "DisplayName" "RcloneDrive"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "DisplayIcon" "$INSTDIR\RcloneDrive.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive" "NoRepair" 1
SectionEnd

Section "WinFsp（未安装时才执行）" SEC_WINFSP
  ; 已安装（驱动服务已注册）则跳过。WinFsp 2.x 驱动服务名带时间戳后缀
  ; （WinFsp+2026...），不能查固定键名，枚举服务项按前缀匹配（兼容 1.x）。
  StrCpy $2 0
check_svc:
  EnumRegKey $1 HKLM "SYSTEM\CurrentControlSet\Services" $2
  StrCmp $1 "" check_file
  StrCpy $3 $1 6
  StrCmp $3 "WinFsp" skip_install
  IntOp $2 $2 + 1
  Goto check_svc
check_file:
  ; 兜底：按默认安装位置查驱动 DLL（服务名未来再变也能识别）
  IfFileExists "$PROGRAMFILES\WinFsp\bin\winfsp-x64.dll" skip_install
  Goto do_install
skip_install:
  DetailPrint "WinFsp 已安装，跳过"
  Goto winfsp_done
do_install:
  SetOutPath $PLUGINSDIR
  File /oname=winfsp.msi "${WINFSP_MSI}"
  DetailPrint "正在静默安装 WinFsp…"
  ExecWait 'msiexec /i "$PLUGINSDIR\winfsp.msi" /qn /norestart' $1
  IntCmp $1 0 winfsp_done
  IntCmp $1 3010 need_reboot
    MessageBox MB_OK|MB_ICONEXCLAMATION "WinFsp 安装失败（msiexec 退出码 $1）。挂载云盘需要 WinFsp，可稍后重试。" /SD IDOK
    Goto winfsp_done
  need_reboot:
    SetRebootFlag true
winfsp_done:
SectionEnd

Function CreateDesktopShortcut
  CreateShortCut "$DESKTOP\RcloneDrive.lnk" "$INSTDIR\RcloneDrive.exe"
FunctionEnd

; 经 explorer 中转降权启动：explorer 以桌面会话（非提权）token 拉起应用后退出。
; 已知限制：explorer 未运行的极简环境（Server Core）拉不起应用，此时用户可
; 手动双击快捷方式启动。
Function LaunchUnelevated
  Exec 'explorer.exe "$INSTDIR\RcloneDrive.exe"'
FunctionEnd

; Explorer 会在快捷方式出现后几秒才打开它，删除需重试。
Function un.DeleteShortcut
  Exch $0
  Push $1
  StrCpy $1 50
  retry:
    ClearErrors
    Delete $0
    IfErrors 0 done
    IntOp $1 $1 - 1
    IntCmp $1 0 done
    Sleep 100
    Goto retry
  done:
  Pop $1
  Pop $0
FunctionEnd

Section "Uninstall"
  ; 结束运行中的实例，否则 exe 文件锁导致删除失败（同安装 Section）
  ExecWait 'taskkill /IM RcloneDrive.exe /F'
  Sleep 500
  Push "$SMPROGRAMS\RcloneDrive.lnk"
  Call un.DeleteShortcut
  Push "$DESKTOP\RcloneDrive.lnk"
  Call un.DeleteShortcut
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\com.younland.rclonedrive"
  ; WinFsp 保留不卸：系统组件，其他应用可能也在使用。
  RMDir /r "$INSTDIR"
SectionEnd

; Veloverse Windows Installer (NSIS)
; Build on Linux or Windows once Unreal has packaged the Windows/ directory:
;   makensis -DSRC_DIR=/path/to/Windows -DVERSION=0.8.4 scripts/veloverse.nsi
!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "0.8.4"
!endif
!ifndef SRC_DIR
  !define SRC_DIR "..\out\Windows"
!endif

Name "Veloverse ${VERSION}"
OutFile "Veloverse-Setup-${VERSION}.exe"
Unicode True
InstallDir "$LOCALAPPDATA\Programs\Veloverse"
InstallDirRegKey HKCU "Software\Veloverse" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\VeloverseUE.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Veloverse"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Veloverse" SecMain
  SetOutPath "$INSTDIR"
  File /r "${SRC_DIR}\*.*"

  WriteRegStr HKCU "Software\Veloverse" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Veloverse" "DisplayName" "Veloverse"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Veloverse" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Veloverse" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\Veloverse"
  CreateShortcut "$SMPROGRAMS\Veloverse\Veloverse.lnk" "$INSTDIR\VeloverseUE.exe"
  CreateShortcut "$SMPROGRAMS\Veloverse\Uninstall Veloverse.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortcut "$DESKTOP\Veloverse.lnk" "$INSTDIR\VeloverseUE.exe"
SectionEnd

Section "Uninstall"
  Delete "$DESKTOP\Veloverse.lnk"
  RMDir /r "$SMPROGRAMS\Veloverse"
  RMDir /r "$INSTDIR"
  DeleteRegKey HKCU "Software\Veloverse"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Veloverse"
SectionEnd

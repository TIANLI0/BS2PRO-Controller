Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## Override to prevent duplicate product names in registry key
!define UNINST_KEY_NAME "BS2PRO-Controller"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# Include required plugins and libraries
!include "MUI.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"

# Include .NET Framework Detection
!include "DotNetChecker.nsh"

# Built-in PawnIO version for upgrade/repair decisions.
# You can override this at build time with: -DPAWNIO_BUNDLED_VERSION=x.y.z
!ifndef PAWNIO_BUNDLED_VERSION
!define PAWNIO_BUNDLED_VERSION "2.2.0.0"
!endif

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch BS2PRO Controller after installation"
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_COMPONENTS # Component selection page
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
Caption "${INFO_PRODUCTNAME} Installer v${INFO_PRODUCTVERSION}"
BrandingText "${INFO_PRODUCTNAME} v${INFO_PRODUCTVERSION}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}" # Default installing folder (single level)
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
   
   # Check for .NET Framework 4.7.2 or later
   !insertmacro CheckNetFramework 472
   Pop $0
   ${If} $0 == "false"
       MessageBox MB_OK|MB_ICONSTOP ".NET Framework 4.7.2 or later is required.$\n$\nPlease install .NET Framework 4.7.2 first."
       Abort
   ${EndIf}
   
   # Stop running instances first to avoid file locks
   Call StopRunningInstances
   
   # Check for existing installation and set install directory
   Call DetectExistingInstallation
FunctionEnd

# Function to clean up legacy/duplicate registry keys
Function CleanLegacyRegistryKeys
    DetailPrint "Cleaning up legacy registry keys..."
    SetRegView 64
    
    # List of known legacy/duplicate registry key names
    # BS2PRO-controllerBS2PRO-controller (duplicate product name)
    # TIANLI0BS2PRO-Controller (old company+product format)
    # BS2PRO-ControllerBS2PRO-Controller (case variation)
    
    Push $R0
    Push $R1
    
    # Check and remove BS2PRO-controllerBS2PRO-controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "Found duplicate registry key: BS2PRO-controllerBS2PRO-controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller"
        DetailPrint "Deleted duplicate registry key"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO-Controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "Found legacy registry key: TIANLI0BS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller"
        DetailPrint "Deleted legacy registry key"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO (current wails.json would generate this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "Found duplicate registry key: TIANLI0BS2PRO"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO"
        DetailPrint "Deleted duplicate registry key"
    ${EndIf}
    
    Pop $R1
    Pop $R0
FunctionEnd

# Function to detect existing installation and set install directory
Function DetectExistingInstallation
    DetailPrint "Checking for existing installation..."
    SetRegView 64
    
    Push $R0
    Push $R1
    Push $R2

    # Show locally installed version if available
    ReadRegStr $R2 HKLM "${UNINST_KEY}" "DisplayVersion"
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "DisplayVersion"
    ${EndIf}
    ${If} $R2 != ""
        DetailPrint "Locally installed version: $R2"
    ${Else}
        DetailPrint "No locally installed version detected"
    ${EndIf}
    
    # First, check all possible registry keys to find installation path
    # DO NOT delete registry keys yet - we need them to find the install path!
    
    # Method 1: Try current/correct registry key (BS2PRO-Controller)
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "Found existing installation (correct key - install location): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "Found existing installation (correct key - install location - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "${UNINST_KEY}" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found existing installation (from correct registry key): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found existing installation (from correct registry key - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Method 2: Check legacy/duplicate registry keys to find old installation
    # BS2PRO-controllerBS2PRO-controller (the current problematic key)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "InstallLocation"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (duplicate key - install location): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (duplicate key - install location - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (duplicate key): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (duplicate key - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Try DisplayIcon from duplicate key
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (from icon path): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (from icon path - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Method 3: Check TIANLI0BS2PRO-Controller (old company+product format)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "InstallLocation"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (old format key - install location): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (old format key - install location - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (old format key): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (old format key - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Method 4: Check TIANLI0BS2PRO (wails.json generates this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "InstallLocation"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (TIANLI0BS2PRO - install location): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "Found legacy installation (TIANLI0BS2PRO - install location - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (TIANLI0BS2PRO): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found legacy installation (TIANLI0BS2PRO - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Second, try to read from DisplayIcon in uninstall registry
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "DisplayIcon"
    ${If} $R0 != ""
        # Remove surrounding quotes
        Push $R0
        Call TrimQuotes
        Pop $R0
        
        ${GetParent} $R0 $R1  # Get parent directory
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "Found existing installation (from icon): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "Found existing installation (from icon - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Third, try to read InstallLocation from registry
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "Found existing installation (from install location): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "Found existing installation (from install location - Core): $INSTDIR"
            Goto found_installation
        ${EndIf}
    ${EndIf}
    
    # Fourth, check common installation locations (single level path)
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "Found existing installation: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\${INFO_PRODUCTNAME}"
        DetailPrint "Found existing installation: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Fifth, check legacy paths with Company\Product structure
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
        DetailPrint "Found existing installation (legacy path): $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Sixth, try alternative common paths
    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "Found existing installation: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\BS2PRO-Controller\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\BS2PRO-Controller"
        DetailPrint "Found existing installation: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Seventh, check for BS2PRO-Core.exe in common paths
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "Found existing installation (Core): $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "Found existing installation (Core): $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # If no existing installation found, use simple product name for directory
    # Use BS2PRO-Controller instead of ${INFO_PRODUCTNAME} to ensure consistency
    StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
    DetailPrint "No existing installation found, using default directory: $INSTDIR"
    Goto end_detection
    
    found_installation:
    DetailPrint "Existing installation detected - will upgrade to: $INSTDIR"
    # Now clean up legacy registry keys AFTER we've found the install path
    Call CleanLegacyRegistryKeys
    
    end_detection:
    Pop $R2
    Pop $R1
    Pop $R0
FunctionEnd

# Function to write current version info to uninstall registry key
Function WriteCurrentVersionInfo
    SetRegView 64
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "Version" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    DetailPrint "Version info written: ${INFO_PRODUCTVERSION}"
FunctionEnd

# Helper function to trim quotes from a string
Function TrimQuotes
    Exch $R0 ; Original string
    Push $R1
    Push $R2
    
    StrCpy $R1 $R0 1 ; First char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 "" 1 ; Remove first quote
    
    StrLen $R2 $R0
    IntOp $R2 $R2 - 1
    StrCpy $R1 $R0 1 $R2 ; Last char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 $R2 ; Remove last quote
    
    Pop $R2
    Pop $R1
    Exch $R0 ; Trimmed string
FunctionEnd

# Function to stop running instances and services
Function StopRunningInstances
    DetailPrint "Checking for running processes..."
    
    # Try to stop the core service first (it manages the fan control)
    # Use /FI with proper error handling
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "Requested shutdown of BS2PRO-Core.exe..."
        Sleep 2000
    ${EndIf}
    
    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1

    # Stop conflicting SpaceStation service process
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "SpaceStationService.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "Requested shutdown of SpaceStationService.exe..."
        Sleep 1000
    ${EndIf}

    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "SpaceStationService.exe" /T'
    Pop $0
    Pop $1
    
    # Try to stop the main application gracefully first
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "Requested shutdown of ${PRODUCT_EXECUTABLE}..."
        Sleep 2000
    ${EndIf}
    
    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    # Backward compatibility: kill legacy main executable names
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO.exe" /T'
    Pop $0
    Pop $1
    
    # Stop any bridge processes (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1

    # Stop and remove kernel driver service that may lock TempBridge.sys
    Call StopBridgeDriver
    
    # Remove scheduled task if exists (ignore errors)
    DetailPrint "Cleaning up scheduled tasks..."
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    
    # Wait a moment for processes to fully terminate
    DetailPrint "Waiting for processes to fully terminate..."
    Sleep 2000
    
    DetailPrint "Process cleanup completed"
FunctionEnd

# Function to stop and remove TempBridge kernel driver service
Function StopBridgeDriver
    DetailPrint "Stopping driver service R0TempBridge..."

    # Stop running driver service (ignore failures if service does not exist)
    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0TempBridge"'
    Pop $0
    Pop $1
    Sleep 1200

    # Delete service entry to release lock for overwrite during upgrade
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0TempBridge"'
    Pop $0
    Pop $1

    # Compatibility: other possible driver service names used by hardware monitor libs
    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0LibreHardwareMonitor"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0LibreHardwareMonitor"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0WinRing0"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0WinRing0"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "WinRing0_1_2_0"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "WinRing0_1_2_0"'
    Pop $0
    Pop $1

    # PawnIO service cleanup (for upgrades / failed previous installs)
    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "PawnIO"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "PawnIO"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0PawnIO"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0PawnIO"'
    Pop $0
    Pop $1
    Sleep 800

    # Best effort delete of driver file in current install dir
    ${If} ${FileExists} "$INSTDIR\bridge\TempBridge.sys"
        Delete /REBOOTOK "$INSTDIR\bridge\TempBridge.sys"
    ${EndIf}
FunctionEnd

# Uninstall-side function (NSIS requires un.* functions in uninstall section)
Function un.StopBridgeDriver
    DetailPrint "Stopping driver service R0TempBridge..."

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0TempBridge"'
    Pop $0
    Pop $1
    Sleep 1200

    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0TempBridge"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0LibreHardwareMonitor"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0LibreHardwareMonitor"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0WinRing0"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0WinRing0"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "WinRing0_1_2_0"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "WinRing0_1_2_0"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "PawnIO"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "PawnIO"'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0PawnIO"'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0PawnIO"'
    Pop $0
    Pop $1
    Sleep 800

    ${If} ${FileExists} "$INSTDIR\bridge\TempBridge.sys"
        Delete /REBOOTOK "$INSTDIR\bridge\TempBridge.sys"
    ${EndIf}
FunctionEnd

# Function to backup user data before upgrade
Function BackupUserData
    DetailPrint "Backing up user configuration..."
    
    # Backup configuration files if they exist
    ${If} ${FileExists} "$INSTDIR\config.json"
        CopyFiles "$INSTDIR\config.json" "$TEMP\bs2pro_config_backup.json"
        DetailPrint "Configuration file backed up"
    ${EndIf}
    
    # Backup other important user files if needed
    ${If} ${FileExists} "$INSTDIR\settings.ini"
        CopyFiles "$INSTDIR\settings.ini" "$TEMP\bs2pro_settings_backup.ini"
        DetailPrint "Settings file backed up"
    ${EndIf}
FunctionEnd

# Function to restore user data after upgrade
Function RestoreUserData
    DetailPrint "Restoring user configuration..."
    
    # Restore configuration files if backup exists
    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        CopyFiles "$TEMP\bs2pro_config_backup.json" "$INSTDIR\config.json"
        DetailPrint "Configuration file restored"
    ${EndIf}
    
    ${If} ${FileExists} "$TEMP\bs2pro_settings_backup.ini"
        CopyFiles "$TEMP\bs2pro_settings_backup.ini" "$INSTDIR\settings.ini"
        Delete "$TEMP\bs2pro_settings_backup.ini"  # Clean up backup
        DetailPrint "Settings file restored"
    ${EndIf}
FunctionEnd

# Uninstall currently installed PawnIO via bundled installer
Function UninstallPawnIO
    DetailPrint "Uninstalling existing PawnIO..."

    ${If} ${FileExists} "$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe"
        nsExec::ExecToStack /TIMEOUT=60000 '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -uninstall -silent'
        Pop $0
        Pop $1

        ${If} $0 == "timeout"
            DetailPrint "PawnIO silent uninstall timed out after 60 seconds, falling back to interactive uninstall..."
            nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "PawnIO_setup.exe" /T'
            Pop $2
            Pop $3
            ExecWait '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -uninstall' $0
            ${If} $0 != 0
                DetailPrint "PawnIO interactive uninstall return code: $0"
            ${EndIf}
        ${ElseIf} $0 == 0
            DetailPrint "PawnIO uninstall completed (silent)"
        ${Else}
            DetailPrint "PawnIO silent uninstall failed, falling back to interactive uninstall..."
            ExecWait '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -uninstall' $0
            ${If} $0 != 0
                DetailPrint "PawnIO interactive uninstall return code: $0"
            ${EndIf}
        ${EndIf}
    ${Else}
        DetailPrint "PawnIO_setup.exe not found, skipping uninstall call"
    ${EndIf}

    # Clean up driver services after uninstall to avoid upgrade leftovers
    Call StopBridgeDriver
    Sleep 1000
FunctionEnd

Section "Main Program (Required)" SEC_MAIN
    SectionIn RO  # Read-only, cannot be deselected
    !insertmacro wails.setShellContext

    # Check if this is an upgrade installation
    ${If} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
        DetailPrint "Upgrading: $INSTDIR"
        
        # Backup important files before upgrade
        Call BackupUserData
        
        # Ensure old instances are completely stopped before upgrading
        Call StopRunningInstances
        
        # Clean up old files but preserve user data
        DetailPrint "Cleaning up old version files..."
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\BS2PRO-Core.exe"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.log"  # Keep log structure but remove old logs
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-Core.exe"
        DetailPrint "Upgrading (found Core): $INSTDIR"
        
        # Backup important files before upgrade
        Call BackupUserData
        
        # Ensure old instances are completely stopped before upgrading
        Call StopRunningInstances
        
        # Clean up old files but preserve user data
        DetailPrint "Cleaning up old version files..."
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\BS2PRO-Core.exe"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.log"
    ${Else}
        DetailPrint "Fresh installation: $INSTDIR"
        
        # Ensure old instances are completely stopped before installing
        Call StopRunningInstances
        
        # Clean up any leftover files from previous installation
        DetailPrint "Cleaning up leftover files..."
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.*"
    ${EndIf}
    
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files
    
    # Copy core service executable
    DetailPrint "Installing core service..."
    File "..\..\bin\BS2PRO-Core.exe"
    
    # Copy bridge directory and its contents
    DetailPrint "Installing bridge components..."
    SetOutPath $INSTDIR\bridge
    File /r "..\..\bin\bridge\*.*"
    
    # Return to main install directory
    SetOutPath $INSTDIR
    
    # Restore user data if this was an upgrade
    Call RestoreUserData

    # Create shortcuts
    DetailPrint "Creating shortcuts..."
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
    Call WriteCurrentVersionInfo
    
    DetailPrint "Installation completed"
    
    # Show completion message
    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        MessageBox MB_OK|MB_ICONINFORMATION "BS2PRO Controller upgraded successfully!$\n$\nYour settings have been preserved.$\n$\nNote: Full functionality requires administrator privileges."
        Delete "$TEMP\bs2pro_config_backup.json"  # Clean up backup
    ${Else}
        MessageBox MB_OK|MB_ICONINFORMATION "BS2PRO Controller installed successfully!$\n$\nNote: Full functionality requires administrator privileges."
    ${EndIf}
SectionEnd

# Auto-start section (selected by default)
Section "Auto-start on boot" SEC_AUTOSTART
    DetailPrint "Configuring auto-start on boot..."
    
    # First, remove any existing auto-start entries to ensure clean state
    DetailPrint "Cleaning up existing auto-start entries..."
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    
    # Create new scheduled task for auto-start with admin privileges
    DetailPrint "Creating auto-start scheduled task..."
    
    # Use schtasks to create a task that runs at logon with highest privileges
    # The task will start BS2PRO-Core.exe with --autostart flag after 15 seconds delay
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /create /tn "BS2PRO-Controller" /tr "\"$INSTDIR\BS2PRO-Core.exe\" --autostart" /sc onlogon /delay 0000:15 /rl highest /f'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "Auto-start configured successfully (scheduled task)"
    ${Else}
        DetailPrint "Scheduled task creation failed, using registry method..."
        # Fallback: use registry auto-start (will trigger UAC on each login)
        WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller" '"$INSTDIR\BS2PRO-Core.exe" --autostart'
        DetailPrint "Auto-start configured successfully (registry)"
    ${EndIf}
SectionEnd

# Required PawnIO installer section
Section "Install PawnIO (Required)" SEC_PAWNIO
    SectionIn RO
    DetailPrint "Preparing to install PawnIO..."
    Push $6
    Push $7
    Push $8
    Push $9

    SetOutPath "$TEMP\BS2PRO-PawnIO"
    File /nonfatal "..\..\bin\PawnIO_setup.exe"
    ${IfNot} ${FileExists} "$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe"
        MessageBox MB_OK|MB_ICONSTOP "PawnIO_setup.exe not found (build\\bin). Please run build_bridge.bat to download it before building the installer."
        Abort
    ${EndIf}

    # Detect installed PawnIO version
    StrCpy $6 ""
    SetRegView 64
    ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
    ${If} $6 == ""
        SetRegView 32
        ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
    ${EndIf}
    SetRegView 64

    # Decide install strategy:
    # $9 = 0 skip, 1 fresh install, 2 upgrade/repair (uninstall then install)
    StrCpy $9 "1"

    ${If} $6 != ""
        DetailPrint "Detected installed PawnIO (version: $6), bundled version: ${PAWNIO_BUNDLED_VERSION}"
        ${VersionCompare} "$6" "${PAWNIO_BUNDLED_VERSION}" $8

        ${If} $8 == 2
            MessageBox MB_YESNO|MB_ICONQUESTION "Detected older PawnIO version: $6.$\nBundled version: ${PAWNIO_BUNDLED_VERSION}.$\n$\nUninstall the old version and install the new one?" IDYES pawnio_upgrade IDNO pawnio_skip
            pawnio_upgrade:
                StrCpy $9 "2"
                Goto pawnio_apply
            pawnio_skip:
                StrCpy $9 "0"
                Goto pawnio_apply
        ${Else}
            MessageBox MB_YESNO|MB_ICONQUESTION "PawnIO is already installed (version: $6).$\n$\nPerform PawnIO repair installation (uninstall then reinstall)?" IDYES pawnio_repair IDNO pawnio_skip2
            pawnio_repair:
                StrCpy $9 "2"
                Goto pawnio_apply
            pawnio_skip2:
                StrCpy $9 "0"
                Goto pawnio_apply
        ${EndIf}
    ${EndIf}

    pawnio_apply:
    ${If} $9 == "0"
        DetailPrint "User chose to skip PawnIO processing."
        Goto pawnio_done
    ${EndIf}

    ${If} $9 == "2"
        Call UninstallPawnIO
    ${EndIf}

    # Pre-clean possible stale driver service state (avoids driver install error 1072)
    DetailPrint "Cleaning up old PawnIO driver services..."
    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "PawnIO"'
    Pop $4
    Pop $5
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "PawnIO"'
    Pop $4
    Pop $5
    nsExec::ExecToStack '"$SYSDIR\sc.exe" stop "R0PawnIO"'
    Pop $4
    Pop $5
    nsExec::ExecToStack '"$SYSDIR\sc.exe" delete "R0PawnIO"'
    Pop $4
    Pop $5
    Sleep 1200

    DetailPrint "Installing PawnIO silently (up to 60 seconds)..."
    nsExec::ExecToStack /TIMEOUT=60000 '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -install -silent'
    Pop $0
    Pop $1
    ${If} $0 == "timeout"
        DetailPrint "PawnIO silent install timed out after 60 seconds, falling back to interactive install..."
        nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "PawnIO_setup.exe" /T'
        Pop $2
        Pop $3
        ExecWait '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -install' $0
        ${If} $0 == 0
            DetailPrint "PawnIO installation completed (interactive)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "PawnIO interactive install failed (return code: $0).$\n$\nCommon cause: driver service marked for deletion by system (error 1072).$\nPlease restart the system and run the installer again."
            Abort
        ${EndIf}
    ${ElseIf} $0 == 0
        DetailPrint "PawnIO installation completed (silent)"
    ${Else}
        DetailPrint "PawnIO silent install failed, switching to interactive install..."
        ExecWait '"$TEMP\BS2PRO-PawnIO\PawnIO_setup.exe" -install' $0
        ${If} $0 == 0
            DetailPrint "PawnIO installation completed (interactive)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "PawnIO installation failed (return code: $0).$\n$\nCommon cause: driver service marked for deletion by system (error 1072).$\nPlease restart the system and run the installer again."
            Abort
        ${EndIf}
    ${EndIf}

    pawnio_done:
    Pop $9
    Pop $8
    Pop $7
    Pop $6
SectionEnd

# Section descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_MAIN} "BS2PRO Controller main program and core service files."
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_AUTOSTART} "Automatically run BS2PRO Controller core service at system startup. Recommended."
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_PAWNIO} "Install PawnIO driver, used for obtaining hardware information."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    # Stop running instances before uninstalling
    DetailPrint "Stopping running processes..."
    
    # Stop core service first (ignore errors)
    DetailPrint "Stopping BS2PRO-Core.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    
    # Stop main application (ignore errors)
    DetailPrint "Stopping ${PRODUCT_EXECUTABLE}..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    # Backward compatibility: stop legacy main executable names
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO.exe" /T'
    Pop $0
    Pop $1
    
    # Stop bridge processes (ignore errors)
    DetailPrint "Stopping TempBridge.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1

    # Stop and remove kernel driver service to release TempBridge.sys lock
    Call un.StopBridgeDriver
    
    # Remove auto-start entries
    DetailPrint "Removing auto-start entries..."
    
    # Remove scheduled task (ignore errors if not exists)
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    
    # Remove registry auto-start entry (both current user and local machine)
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    
    # Remove from startup folder if exists
    Delete "$SMSTARTUP\BS2PRO-Controller.lnk"
    Delete "$SMSTARTUP\BS2PRO-Core.lnk"
    
    # Wait for processes to fully terminate
    Sleep 2000

    # Remove application data directories
    DetailPrint "Removing application data..."
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath
    RMDir /r "$APPDATA\BS2PRO-Controller"
    RMDir /r "$LOCALAPPDATA\BS2PRO-Controller"
    RMDir /r "$TEMP\BS2PRO-Controller"

    # Remove installation directory and all contents
    DetailPrint "Removing installation files..."
    
    # Remove bridge directory (contains TempBridge.exe and related files)
    DetailPrint "Deleting bridge components..."
    RMDir /r "$INSTDIR\bridge"
    
    # Remove logs directory
    DetailPrint "Deleting log files..."
    RMDir /r "$INSTDIR\logs"
    
    # Remove entire installation directory
    DetailPrint "Deleting installation directory..."
    RMDir /r $INSTDIR

    # Remove shortcuts
    DetailPrint "Removing shortcuts..."
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
    
    DetailPrint "Uninstallation completed"
    
    # Optional: Ask user if they want to remove configuration files
    MessageBox MB_YESNO|MB_ICONQUESTION "Delete all configuration files and logs?" IDNO skip_config
    RMDir /r "$APPDATA\BS2PRO"
    RMDir /r "$LOCALAPPDATA\BS2PRO"
    skip_config:
SectionEnd

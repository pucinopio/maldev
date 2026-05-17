@echo off
REM Build script for testlib_msvc.dll — runs inside the Win10 VM
REM under the VS Build Tools "Developer Command Prompt" environment.
REM
REM Invocation pattern from the host (Linux):
REM   scp testlib_msvc.{c,def,build_testlib_msvc.cmd} test@VM:C:/Users/Public/
REM   ssh test@VM "call vcvars64.bat && C:\Users\Public\build_testlib_msvc.cmd"
REM
REM Output: testlib_msvc.dll, testlib_msvc.lib, testlib_msvc.exp.
REM The .dll is the artefact we check into pe/packer/testdata/.
REM
REM Compile flags chosen to exercise the realistic MSVC surface
REM the native-DLL packer must handle: /MD (dynamic CRT imports),
REM /GS (stack cookies → .gfids data), /Gy (function-level COMDAT,
REM produces tight .pdata entries).
setlocal
cl /nologo /LD /MD /O2 /GS /Gy %~dp0testlib_msvc.c ^
   /Fo%~dp0testlib_msvc.obj ^
   /Fe:%~dp0testlib_msvc.dll ^
   /link /NOLOGO /DEF:%~dp0testlib_msvc.def
exit /b %ERRORLEVEL%

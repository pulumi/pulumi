@echo off
powershell -noprofile -executionPolicy Unrestricted -file "%~dpn0.ps1" %*
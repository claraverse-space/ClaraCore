# 🔒 Security Verification Guide

## Why is ClaraCore Flagged by Antivirus?

**ClaraCore is 100% safe and open source.** Antivirus software flags it because it:
1. Downloads AI model binaries from llama.cpp (legitimate ML tool)
2. Spawns subprocesses to run AI models
3. Self-heals by downloading missing components

These are **normal behaviors for AI inference tools**, but trigger heuristic detection.

## ✅ Verify ClaraCore is Legitimate

### Method 1: Check SHA256 Hash (Recommended)

**Windows:**
```powershell
# Calculate hash of your downloaded file
Get-FileHash claracore.exe -Algorithm SHA256

# Compare with official release hash at:
# https://github.com/claraverse-space/ClaraCore/releases
```

**Linux/macOS:**
```bash
sha256sum claracore
# Compare with official release hash
```

### Method 2: Build from Source (Ultimate Trust)

```bash
# Clone and inspect the code yourself
git clone https://github.com/claraverse-space/ClaraCore.git
cd ClaraCore

# Review the code (it's all open source!)
# Then build it yourself
python build.py
```

### Method 3: VirusTotal Scan

1. Visit https://www.virustotal.com
2. Upload your `claracore.exe`
3. Check that major vendors (Microsoft, Kaspersky, etc.) show it as clean
4. Some generic heuristic engines may flag it - this is normal

## 🛡️ Add Windows Defender Exclusion

**Option A - Using GUI:**
1. Open Windows Security
2. Virus & threat protection → Manage settings
3. Exclusions → Add an exclusion
4. Add folder: `%LOCALAPPDATA%\ClaraCore`

**Option B - PowerShell (Admin required):**
```powershell
Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\ClaraCore"
```

## 📝 What ClaraCore Does NOT Do

- ❌ Does NOT modify Windows Registry
- ❌ Does NOT access system files
- ❌ Does NOT require admin rights
- ❌ Does NOT connect to suspicious servers (only GitHub and llama.cpp)
- ❌ Does NOT modify system settings
- ❌ Does NOT install services or drivers

## 🔍 What ClaraCore DOES Do

- ✅ Downloads llama.cpp binaries from official GitHub releases
- ✅ Creates config files in its own directory
- ✅ Runs local AI models on your computer
- ✅ Provides a web interface on localhost:5800

## 🏢 For Enterprise/Corporate Users

If your IT department blocks ClaraCore:

1. **Submit to your security team for review** - The code is open source
2. **Build from source** - Your security team can audit the code
3. **Use Docker containers** - Alternative deployment method
4. **Request whitelisting** - Based on the official GitHub repository

## 📧 Still Concerned?

- 🔍 Review the source code: https://github.com/claraverse-space/ClaraCore
- 💬 Open an issue: https://github.com/claraverse-space/ClaraCore/issues
- 📖 Read the documentation: https://github.com/claraverse-space/ClaraCore/blob/main/README.md

## 🎯 Why We Don't Code Sign

Code signing certificates cost $200-500/year for individual developers. As an open-source project:
- We rely on **source code transparency** instead
- You can **verify hashes** against official releases
- You can **build from source** for ultimate trust
- We submit to **VirusTotal** and **Microsoft SmartScreen** for reputation building

**Open source is the ultimate code signing.**
